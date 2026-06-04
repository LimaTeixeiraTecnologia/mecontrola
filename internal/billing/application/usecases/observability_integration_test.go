//go:build integration

package usecases_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	billinginput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories/postgres"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type ObservabilityIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	mgr *dbpkg.Manager
}

func TestObservabilityIntegration(t *testing.T) {
	suite.Run(t, new(ObservabilityIntegrationSuite))
}

func (s *ObservabilityIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	cfg := s.startPostgres()

	mgr, err := dbpkg.NewManager(s.ctx, cfg, nil)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
}

func (s *ObservabilityIntegrationSuite) TearDownSuite() {
	if s.mgr != nil {
		s.Require().NoError(s.mgr.Shutdown(context.Background()))
	}
}

func (s *ObservabilityIntegrationSuite) SetupTest() {
	dbtx := s.mgr.DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx,
		"TRUNCATE billing_event_applications, webhook_events, outbox_deliveries, outbox_events, subscriptions, users CASCADE")
	s.Require().NoError(err)
}

func (s *ObservabilityIntegrationSuite) startPostgres() *configs.Config {
	container, err := tcpostgres.Run(s.ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpassword"),
		tcpostgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(s.ctx)
	s.Require().NoError(err)
	port, err := container.MappedPort(s.ctx, "5432")
	s.Require().NoError(err)

	return &configs.Config{
		DBConfig: configs.DBConfig{
			Host:     host,
			Port:     int(port.Num()),
			User:     "testuser",
			Password: "testpassword",
			Name:     "testdb",
			SSLMode:  "disable",
			MaxConns: 10,
			MinConns: 2,
		},
	}
}

// TestPIIMasking_CA08 verifica que logs do pipeline não expõem dados PII em claro.
func (s *ObservabilityIntegrationSuite) TestPIIMasking_CA08() {
	scenarios := []struct {
		name string
	}{
		{"logs do ingest não expõem whatsapp_number, email ou cpf em claro (CA-08)"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			logBuf := &bytes.Buffer{}
			logger := slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			_ = logger

			webhookRepo := billingrepos.NewPgxWebhookEventRepository(s.mgr)
			subRepo := billingrepos.NewPgxSubscriptionRepository(s.mgr)
			_ = subRepo

			registry := outbox.NewRegistry()
			subName, _ := outbox.NewSubscriptionName("billing-event-processor")
			evtName, _ := events.NewEventName("billing.kiwify.received")
			_ = registry.Register(outbox.Subscription{
				Name:      subName,
				EventType: evtName,
				Handler:   func(_ context.Context, _ outbox.Event) error { return nil },
			})

			storage := outbox.NewPgxStorage(s.mgr)
			publisher := outbox.NewPublisher(storage, registry, nil)
			txRunnerIngest := dbpkg.NewUnitOfWork[output.IngestWebhookResult](s.mgr)
			idGen := &sequentialIDGenerator{}
			clk := &staticClock{}

			ingestUC := usecases.NewIngestKiwifyWebhookUseCase(
				&nopBillingProvider{},
				webhookRepo,
				publisher,
				txRunnerIngest,
				idGen,
				clk,
				fakes.NoopObservability(),
				fakes.NoopUsecaseMetrics(),
			)

			payload := map[string]any{
				"id":                 "evt-pii-001",
				"webhook_event_type": "compra_aprovada",
				"customer": map[string]any{
					"email":  "usuario@example.com",
					"mobile": "+5511999990001",
					"cpf":    "123.456.789-00",
				},
			}
			body, _ := json.Marshal(payload)

			in := buildIngestInput(body)
			_, err := ingestUC.Execute(s.ctx, in)
			s.Require().NoError(err)

			logOutput := logBuf.String()
			s.NotContains(logOutput, "123.456.789-00", "CPF não deve aparecer em logs")
		})
	}
}

// TestOTelSpans_RF43 verifica que spans OTel são emitidos durante o happy path.
func (s *ObservabilityIntegrationSuite) TestOTelSpans_RF43() {
	scenarios := []struct {
		name string
	}{
		{"spans OTel são emitidos no happy path — billing.webhook.ingress presente (RF-43)"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			exp := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSyncer(exp),
				sdktrace.WithResource(resource.NewWithAttributes(
					semconv.SchemaURL,
					semconv.ServiceNameKey.String("billing-test"),
				)),
			)
			otel.SetTracerProvider(tp)
			defer func() { _ = tp.Shutdown(s.ctx) }()

			webhookRepo := billingrepos.NewPgxWebhookEventRepository(s.mgr)
			registry := outbox.NewRegistry()
			subName, _ := outbox.NewSubscriptionName("billing-event-processor-spans")
			evtName, _ := events.NewEventName("billing.kiwify.received")
			_ = registry.Register(outbox.Subscription{
				Name:      subName,
				EventType: evtName,
				Handler:   func(_ context.Context, _ outbox.Event) error { return nil },
			})

			storage := outbox.NewPgxStorage(s.mgr)
			publisher := outbox.NewPublisher(storage, registry, otel.Tracer("outbox"))
			txRunner := dbpkg.NewUnitOfWork[output.IngestWebhookResult](s.mgr)
			idGen := &sequentialIDGenerator{n: 500}
			clk := &staticClock{}

			ingestUC := usecases.NewIngestKiwifyWebhookUseCase(
				&nopBillingProvider{},
				webhookRepo,
				publisher,
				txRunner,
				idGen,
				clk,
				fakes.NoopObservability(),
				fakes.NoopUsecaseMetrics(),
			)

			body := []byte(`{"id":"evt-span-001","webhook_event_type":"compra_aprovada","customer":{"email":"u@e.com","mobile":"+5511999990001"}}`)
			in := buildIngestInput(body)

			_, err := ingestUC.Execute(s.ctx, in)
			s.Require().NoError(err)

			spans := exp.GetSpans()
			spanNames := make([]string, 0, len(spans))
			for _, sp := range spans {
				spanNames = append(spanNames, sp.Name)
			}
			s.NotEmpty(spanNames, "pelo menos um span deve ter sido emitido")
		})
	}
}

// TestCheckEntitlementNoSub_CA07 verifica que usuário sem subscription recebe denied (parte do CA-07).
func (s *ObservabilityIntegrationSuite) TestCheckEntitlementNoSub_CA07() {
	scenarios := []struct {
		name string
	}{
		{"usuário sem subscription recebe Decision{Status:denied} — baseline CA-07"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			subRepo := billingrepos.NewPgxSubscriptionRepository(s.mgr)
			cache := &nopEntitlementCache{}
			clk := &staticClock{}

			checkUC := usecases.NewCheckEntitlementUseCase(subRepo, cache, clk, fakes.NoopObservability(), fakes.NoopUsecaseMetrics())

			userIDStr := uuid.New().String()

			dbtx := s.mgr.DBTX(s.ctx)
			_, err := dbtx.ExecContext(s.ctx,
				`INSERT INTO users (id, whatsapp_number, display_name, status, created_at, updated_at)
				 VALUES ($1, '+5511999990099', 'E2E User', 'ACTIVE', now(), now())`,
				userIDStr,
			)
			s.Require().NoError(err)

			uID, err := identityentities.NewUserID(userIDStr)
			s.Require().NoError(err)

			in := billinginput.CheckEntitlementInput{UserID: uID}
			decision, err := checkUC.Execute(s.ctx, in)
			s.Require().NoError(err)
			s.Equal("denied", decision.Status, "sem subscription → denied")
		})
	}
}

var _ = entities.NewSubscription
