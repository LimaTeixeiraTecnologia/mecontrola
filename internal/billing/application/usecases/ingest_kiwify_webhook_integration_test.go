//go:build integration

package usecases_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories/postgres"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type IngestKiwifyWebhookIntegrationSuite struct {
	suite.Suite
	ctx      context.Context
	mgr      *dbpkg.Manager
	ingestUC *usecases.IngestKiwifyWebhookUseCase
}

func TestIngestKiwifyWebhookIntegration(t *testing.T) {
	suite.Run(t, new(IngestKiwifyWebhookIntegrationSuite))
}

func (s *IngestKiwifyWebhookIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	cfg := s.startPostgres()

	mgr, err := dbpkg.NewManager(s.ctx, cfg, nil)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
	s.ingestUC = s.buildIngestUC()
}

func (s *IngestKiwifyWebhookIntegrationSuite) TearDownSuite() {
	if s.mgr != nil {
		s.Require().NoError(s.mgr.Shutdown(context.Background()))
	}
}

func (s *IngestKiwifyWebhookIntegrationSuite) SetupTest() {
	dbtx := s.mgr.DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx,
		"TRUNCATE billing_event_applications, webhook_events, outbox_deliveries, outbox_events, subscriptions CASCADE")
	s.Require().NoError(err)
}

func (s *IngestKiwifyWebhookIntegrationSuite) startPostgres() *configs.Config {
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

func (s *IngestKiwifyWebhookIntegrationSuite) buildIngestUC() *usecases.IngestKiwifyWebhookUseCase {
	webhookRepo := billingrepos.NewPgxWebhookEventRepository(s.mgr)

	registry := outbox.NewRegistry()
	subName, err := outbox.NewSubscriptionName("billing-event-processor")
	s.Require().NoError(err)
	evtName, err := events.NewEventName("billing.kiwify.received")
	s.Require().NoError(err)
	s.Require().NoError(registry.Register(outbox.Subscription{
		Name:      subName,
		EventType: evtName,
		Handler:   func(_ context.Context, _ outbox.Event) error { return nil },
	}))

	storage := outbox.NewPgxStorage(s.mgr)
	publisher := outbox.NewPublisher(storage, registry, nil)
	txRunner := dbpkg.NewUnitOfWork[output.IngestWebhookResult](s.mgr)

	return usecases.NewIngestKiwifyWebhookUseCase(
		&nopBillingProvider{},
		webhookRepo,
		publisher,
		txRunner,
		&sequentialIDGenerator{},
		&staticClock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)},
		fakes.NoopObservability(),
		fakes.NoopUsecaseMetrics(),
	)
}

func (s *IngestKiwifyWebhookIntegrationSuite) buildPayload(eventID string) []byte {
	payload := map[string]any{
		"id":                 eventID,
		"webhook_event_type": "compra_aprovada",
		"customer": map[string]any{
			"email":  "user@example.com",
			"mobile": "+5511999990001",
		},
	}
	b, err := json.Marshal(payload)
	s.Require().NoError(err)
	return b
}

func (s *IngestKiwifyWebhookIntegrationSuite) TestIngest_NewPayload_CreatesRowAndOutbox() {
	scenarios := []struct {
		name string
	}{
		{"payload novo cria 1 row em webhook_events e 1 em outbox_events"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			body := s.buildPayload("evt-ingest-001")
			in := input.IngestWebhookInput{
				RawBody: body,
				Headers: map[string]string{"X-Kiwify-Webhook-Token": "test-secret"},
			}
			result, err := s.ingestUC.Execute(s.ctx, in)
			s.Require().NoError(err)
			s.False(result.Duplicate)

			dbtx := s.mgr.DBTX(s.ctx)
			var webhookCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM webhook_events").Scan(&webhookCount))
			s.Equal(1, webhookCount)

			var outboxCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM outbox_events").Scan(&outboxCount))
			s.Equal(1, outboxCount)
		})
	}
}

func (s *IngestKiwifyWebhookIntegrationSuite) TestIngest_ReplayPayload_NoNewRows() {
	scenarios := []struct {
		name string
	}{
		{"replay do mesmo payload não cria novas rows (CA-02 — dedup via ON CONFLICT)"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			body := s.buildPayload("evt-replay-001")
			in := input.IngestWebhookInput{
				RawBody: body,
				Headers: map[string]string{"X-Kiwify-Webhook-Token": "test-secret"},
			}

			result1, err := s.ingestUC.Execute(s.ctx, in)
			s.Require().NoError(err)
			s.False(result1.Duplicate)

			result2, err := s.ingestUC.Execute(s.ctx, in)
			s.Require().NoError(err)
			s.True(result2.Duplicate, "segunda ingestão deve retornar Duplicate=true")

			dbtx := s.mgr.DBTX(s.ctx)
			var count int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM webhook_events WHERE external_event_id = $1",
				"evt-replay-001").Scan(&count))
			s.Equal(1, count, "apenas 1 row deve existir para o external_event_id")
		})
	}
}

// nopBillingProvider aceita qualquer assinatura e não faz parse real — para integration tests
// de ingest que não exercitam o processor.
type nopBillingProvider struct{}

func (*nopBillingProvider) VerifySignature(_ []byte, _ map[string]string) error { return nil }
func (*nopBillingProvider) ParseEvent(_ []byte) (services.CanonicalEvent, error) {
	return services.CanonicalEvent{}, nil
}
func (*nopBillingProvider) FetchSubscription(_ context.Context, _ string) (services.CanonicalSubscription, error) {
	return services.CanonicalSubscription{}, nil
}

// sequentialIDGenerator gera IDs UUID-v4-like determinísticos para testes.
type sequentialIDGenerator struct {
	n int
}

func (g *sequentialIDGenerator) NewID() string {
	g.n++
	return fmt.Sprintf("550e8400-e29b-41d4-a716-%012x", g.n)
}

// staticClock retorna sempre o mesmo instante.
type staticClock struct {
	t time.Time
}

func (c *staticClock) Now() time.Time { return c.t }

// buildIngestInput constrói um IngestWebhookInput a partir de payload bruto para testes de integração.
func buildIngestInput(body []byte) input.IngestWebhookInput {
	return input.IngestWebhookInput{
		RawBody: body,
		Headers: map[string]string{"X-Kiwify-Webhook-Token": "test-secret"},
	}
}

// logCapture captura saída do slog em buffer para validação de PII.
type logCapture struct {
	buf *bytes.Buffer
}

func newLogCapture() (*logCapture, *slog.Logger) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return &logCapture{buf: buf}, logger
}

func (l *logCapture) Contains(s string) bool {
	return bytes.Contains(l.buf.Bytes(), []byte(s))
}
