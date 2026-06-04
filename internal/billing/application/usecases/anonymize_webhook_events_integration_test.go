//go:build integration

package usecases_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	billinginput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories/postgres"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
)

type AnonymizeWebhookEventsIntegrationSuite struct {
	suite.Suite
	ctx         context.Context
	mgr         *dbpkg.Manager
	webhookRepo *billingrepos.PgxWebhookEventRepository
	anonymizeUC *usecases.AnonymizeWebhookEventsUseCase
}

func TestAnonymizeWebhookEventsIntegration(t *testing.T) {
	suite.Run(t, new(AnonymizeWebhookEventsIntegrationSuite))
}

func (s *AnonymizeWebhookEventsIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	cfg := s.startPostgres()

	mgr, err := dbpkg.NewManager(s.ctx, cfg, nil)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
	s.webhookRepo = billingrepos.NewPgxWebhookEventRepository(s.mgr)
	s.anonymizeUC = usecases.NewAnonymizeWebhookEventsUseCase(
		s.webhookRepo,
		services.NewPIIRedactor(),
		&staticClock{t: time.Now().UTC()},
		fakes.NoopObservability(),
		fakes.NoopUsecaseMetrics(),
	)
}

func (s *AnonymizeWebhookEventsIntegrationSuite) TearDownSuite() {
	if s.mgr != nil {
		s.Require().NoError(s.mgr.Shutdown(context.Background()))
	}
}

func (s *AnonymizeWebhookEventsIntegrationSuite) SetupTest() {
	dbtx := s.mgr.DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx,
		"TRUNCATE billing_event_applications, webhook_events, outbox_deliveries, outbox_events, subscriptions CASCADE")
	s.Require().NoError(err)
}

func (s *AnonymizeWebhookEventsIntegrationSuite) startPostgres() *configs.Config {
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

func (s *AnonymizeWebhookEventsIntegrationSuite) insertOldWebhookEvent(webhookID, extEventID string, daysAgo int) {
	payload := json.RawMessage(`{"id":"` + extEventID + `","customer":{"email":"user@example.com","mobile":"+5511999990001","cpf":"123.456.789-00"},"tracking":{"src":"token123"},"product":{"id":"prod-001"}}`)
	extID, err := valueobjects.NewExternalEventIDCascade(payload)
	s.Require().NoError(err)
	wID, err := valueobjects.NewWebhookEventID(webhookID)
	s.Require().NoError(err)

	event, err := entities.NewWebhookEvent(entities.NewWebhookEventParams{
		ID:              wID,
		Provider:        "kiwify",
		ExternalEventID: extID,
		EventType:       "compra_aprovada",
		Signature:       "test-token",
		HeadersJSON:     json.RawMessage(`{}`),
		Payload:         payload,
		ReceivedAt:      time.Now().UTC(),
	})
	s.Require().NoError(err)
	_, err = s.webhookRepo.InsertIfNew(s.ctx, event)
	s.Require().NoError(err)

	dbtx := s.mgr.DBTX(s.ctx)
	_, err = dbtx.ExecContext(s.ctx,
		"UPDATE webhook_events SET received_at = now() - ($1 * INTERVAL '1 day') WHERE id = $2",
		daysAgo, webhookID,
	)
	s.Require().NoError(err)
}

// TestAnonymize_SubstitutesPII_CA12 — CA-12 completo.
func (s *AnonymizeWebhookEventsIntegrationSuite) TestAnonymize_SubstitutesPII_CA12() {
	scenarios := []struct {
		name string
	}{
		{"CA-12: anonymize substitui PII, preserva metadados, preenche anonymized_at, é idempotente"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.insertOldWebhookEvent("550e8400-e29b-41d4-a716-446655440301", "evt-anon-001", 400)

			olderThan := time.Now().UTC().Add(-365 * 24 * time.Hour)
			in := billinginput.AnonymizeInput{OlderThan: olderThan, BatchSize: 100}

			report, err := s.anonymizeUC.Execute(s.ctx, in)
			s.Require().NoError(err)
			s.Equal(1, report.Processed)
			s.Equal(0, report.Errors)

			dbtx := s.mgr.DBTX(s.ctx)
			var payload []byte
			var anonymizedAt *time.Time
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT payload, anonymized_at FROM webhook_events WHERE id = $1",
				"550e8400-e29b-41d4-a716-446655440301").Scan(&payload, &anonymizedAt))

			s.NotNil(anonymizedAt, "(a) anonymized_at deve estar preenchido")

			var doc map[string]any
			s.Require().NoError(json.Unmarshal(payload, &doc))

			customer, ok := doc["customer"].(map[string]any)
			s.True(ok)
			s.Equal("[REDACTED]", customer["email"], "(a) customer.email deve ser [REDACTED]")
			s.Equal("[REDACTED]", customer["mobile"], "(a) customer.mobile deve ser [REDACTED]")
			s.Equal("[REDACTED]", customer["cpf"], "(a) customer.cpf deve ser [REDACTED]")

			tracking, ok := doc["tracking"].(map[string]any)
			s.True(ok)
			s.Equal("token123", tracking["src"], "(b) tracking.src deve ser preservado")

			product, ok := doc["product"].(map[string]any)
			s.True(ok)
			s.Equal("prod-001", product["id"], "(b) product.id deve ser preservado")

			report2, err := s.anonymizeUC.Execute(s.ctx, in)
			s.Require().NoError(err)
			s.Equal(0, report2.Processed, "(d) re-execução deve ser no-op")
		})
	}
}

func (s *AnonymizeWebhookEventsIntegrationSuite) TestAnonymize_RecentRows_NotProcessed() {
	scenarios := []struct {
		name string
	}{
		{"rows com received_at recente (< 365d) não são anonimizadas"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.insertOldWebhookEvent("550e8400-e29b-41d4-a716-446655440302", "evt-anon-002", 10)

			olderThan := time.Now().UTC().Add(-365 * 24 * time.Hour)
			in := billinginput.AnonymizeInput{OlderThan: olderThan, BatchSize: 100}

			report, err := s.anonymizeUC.Execute(s.ctx, in)
			s.Require().NoError(err)
			s.Equal(0, report.Processed, "row recente não deve ser anonimizada")
		})
	}
}
