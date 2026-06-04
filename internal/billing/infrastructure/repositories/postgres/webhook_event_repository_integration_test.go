//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories/postgres"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type WebhookEventRepoIntegrationSuite struct {
	suite.Suite
	ctx         context.Context
	mgr         *dbpkg.Manager
	webhookRepo *billingrepos.PgxWebhookEventRepository
}

func TestWebhookEventRepoIntegration(t *testing.T) {
	suite.Run(t, new(WebhookEventRepoIntegrationSuite))
}

func (s *WebhookEventRepoIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	cfg := s.startPostgres()

	mgr, err := dbpkg.NewManager(s.ctx, cfg, nil)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
	s.webhookRepo = billingrepos.NewPgxWebhookEventRepository(s.mgr)
}

func (s *WebhookEventRepoIntegrationSuite) TearDownSuite() {
	if s.mgr != nil {
		s.Require().NoError(s.mgr.Shutdown(context.Background()))
	}
}

func (s *WebhookEventRepoIntegrationSuite) SetupTest() {
	dbtx := s.mgr.DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx,
		"TRUNCATE billing_event_applications, webhook_events, outbox_deliveries, outbox_events, subscriptions CASCADE")
	s.Require().NoError(err)
}

func (s *WebhookEventRepoIntegrationSuite) startPostgres() *configs.Config {
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

func webhookTestUUID(n int) string {
	return fmt.Sprintf("550e8400-e29b-41d4-a716-%012x", n)
}

func (s *WebhookEventRepoIntegrationSuite) mustWebhookEventID(v string) valueobjects.WebhookEventID {
	id, err := valueobjects.NewWebhookEventID(v)
	s.Require().NoError(err)
	return id
}

func (s *WebhookEventRepoIntegrationSuite) buildWebhookEvent(webhookID, extEventID string) entities.WebhookEvent {
	payload := json.RawMessage(`{"id":"` + extEventID + `","webhook_event_type":"compra_aprovada","customer":{"email":"user@example.com","mobile":"+5511999990001","cpf":"000.000.000-00"},"tracking":{"src":"token123"}}`)
	extID, err := valueobjects.NewExternalEventIDCascade(payload)
	s.Require().NoError(err)

	event, err := entities.NewWebhookEvent(entities.NewWebhookEventParams{
		ID:              s.mustWebhookEventID(webhookID),
		Provider:        "kiwify",
		ExternalEventID: extID,
		EventType:       "compra_aprovada",
		Signature:       "webhook-token-secret",
		HeadersJSON:     json.RawMessage(`{}`),
		Payload:         payload,
		ReceivedAt:      time.Now().UTC(),
	})
	s.Require().NoError(err)
	return event
}

func (s *WebhookEventRepoIntegrationSuite) TestInsertIfNew_Insert() {
	scenarios := []struct {
		name string
	}{
		{"novo webhook event é inserido e retorna inserted=true"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			event := s.buildWebhookEvent(webhookTestUUID(1), "ext-evt-001")
			inserted, err := s.webhookRepo.InsertIfNew(s.ctx, event)
			s.Require().NoError(err)
			s.True(inserted)

			dbtx := s.mgr.DBTX(s.ctx)
			var count int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM webhook_events WHERE id = $1", event.ID().String()).Scan(&count))
			s.Equal(1, count)
		})
	}
}

func (s *WebhookEventRepoIntegrationSuite) TestInsertIfNew_Dedup() {
	scenarios := []struct {
		name string
	}{
		{"mesmo (provider, external_event_id) retorna inserted=false na segunda chamada"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			event := s.buildWebhookEvent(webhookTestUUID(2), "ext-evt-002")
			inserted1, err := s.webhookRepo.InsertIfNew(s.ctx, event)
			s.Require().NoError(err)
			s.True(inserted1)

			event2 := s.buildWebhookEvent(webhookTestUUID(3), "ext-evt-002")
			inserted2, err := s.webhookRepo.InsertIfNew(s.ctx, event2)
			s.Require().NoError(err)
			s.False(inserted2, "duplicata deve retornar inserted=false")

			dbtx := s.mgr.DBTX(s.ctx)
			var count int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM webhook_events WHERE external_event_id = $1 AND provider = 'kiwify'", "ext-evt-002").Scan(&count))
			s.Equal(1, count, "apenas 1 row deve existir para o external_event_id")
		})
	}
}

func (s *WebhookEventRepoIntegrationSuite) insertPrerequisiteUserAndSubscription(userUUID, subID string, webhookEventID valueobjects.WebhookEventID) {
	dbtx := s.mgr.DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx,
		`INSERT INTO users (id, whatsapp_number, display_name, status, created_at, updated_at)
		 VALUES ($1, '+5511999990099', 'Test', 'ACTIVE', now(), now()) ON CONFLICT (id) DO NOTHING`,
		userUUID)
	s.Require().NoError(err)
	_, err = dbtx.ExecContext(s.ctx,
		`INSERT INTO subscriptions (id, user_id, provider, external_subscription_id, plan_code, status,
		  period_start, period_end, grace_period_end, refund_amount_cents,
		  last_event_at, last_webhook_event_id, created_at, updated_at)
		 VALUES ($1, $2, 'kiwify', 'ext-sub-test', 'MONTHLY', 'ACTIVE',
		  now(), now() + interval '30 days', now() + interval '37 days', 0,
		  now(), $3, now(), now()) ON CONFLICT (id) DO NOTHING`,
		subID, userUUID, webhookEventID.String())
	s.Require().NoError(err)
}

func (s *WebhookEventRepoIntegrationSuite) TestRecordApplication_Idempotent() {
	scenarios := []struct {
		name string
	}{
		{"RecordApplication é idempotente — segunda chamada com mesmo event_id retorna false"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			event := s.buildWebhookEvent(webhookTestUUID(10), "ext-evt-010")
			_, err := s.webhookRepo.InsertIfNew(s.ctx, event)
			s.Require().NoError(err)

			subIDStr := "sub-record-app-001"
			s.insertPrerequisiteUserAndSubscription("550e8400-e29b-41d4-a716-446655440099", subIDStr, event.ID())

			subID, err := entities.NewSubscriptionID(subIDStr)
			s.Require().NoError(err)

			now := time.Now().UTC()
			recorded1, err := s.webhookRepo.RecordApplication(s.ctx, event.ID(), subID, now)
			s.Require().NoError(err)
			s.True(recorded1)

			recorded2, err := s.webhookRepo.RecordApplication(s.ctx, event.ID(), subID, now)
			s.Require().NoError(err)
			s.False(recorded2, "segunda aplicação deve retornar false (idempotência)")

			dbtx := s.mgr.DBTX(s.ctx)
			var count int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM billing_event_applications WHERE event_id = $1", event.ID().String()).Scan(&count))
			s.Equal(1, count)
		})
	}
}

func (s *WebhookEventRepoIntegrationSuite) TestListPendingAnonymization_OldRows() {
	scenarios := []struct {
		name string
	}{
		{"ListPendingAnonymization retorna rows com received_at antigo e anonymized_at NULL"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			event := s.buildWebhookEvent(webhookTestUUID(30), "ext-evt-030")
			_, err := s.webhookRepo.InsertIfNew(s.ctx, event)
			s.Require().NoError(err)

			dbtx := s.mgr.DBTX(s.ctx)
			_, err = dbtx.ExecContext(s.ctx,
				"UPDATE webhook_events SET received_at = now() - INTERVAL '400 days' WHERE id = $1",
				event.ID().String())
			s.Require().NoError(err)

			olderThan := time.Now().UTC().Add(-365 * 24 * time.Hour)
			pending, err := s.webhookRepo.ListPendingAnonymization(s.ctx, olderThan, 10)
			s.Require().NoError(err)
			s.Len(pending, 1)
			s.Equal(event.ID().String(), pending[0].ID().String())
		})
	}
}

func (s *WebhookEventRepoIntegrationSuite) TestAnonymize_SubstitutesPII() {
	scenarios := []struct {
		name string
	}{
		{"Anonymize substitui PII e preenche anonymized_at — CA-12"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			event := s.buildWebhookEvent(webhookTestUUID(40), "ext-evt-040")
			_, err := s.webhookRepo.InsertIfNew(s.ctx, event)
			s.Require().NoError(err)

			dbtx := s.mgr.DBTX(s.ctx)
			_, err = dbtx.ExecContext(s.ctx,
				"UPDATE webhook_events SET received_at = now() - INTERVAL '400 days' WHERE id = $1",
				event.ID().String())
			s.Require().NoError(err)

			redacted := json.RawMessage(`{"webhook_event_type":"compra_aprovada","customer":{"email":"[REDACTED]","mobile":"[REDACTED]","cpf":"[REDACTED]"},"tracking":{"src":"token123"}}`)
			now := time.Now().UTC()
			s.Require().NoError(s.webhookRepo.Anonymize(s.ctx, event.ID(), redacted, now))

			var payload []byte
			var anonymizedAt *time.Time
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT payload, anonymized_at FROM webhook_events WHERE id = $1",
				event.ID().String()).Scan(&payload, &anonymizedAt))

			s.NotNil(anonymizedAt, "anonymized_at deve ser preenchido")

			var doc map[string]any
			s.Require().NoError(json.Unmarshal(payload, &doc))

			customer, ok := doc["customer"].(map[string]any)
			s.True(ok)
			s.Equal("[REDACTED]", customer["email"])
			s.Equal("[REDACTED]", customer["mobile"])
			s.Equal("[REDACTED]", customer["cpf"])

			tracking, ok := doc["tracking"].(map[string]any)
			s.True(ok)
			s.Equal("token123", tracking["src"], "campo não-PII deve ser preservado")
		})
	}
}

func (s *WebhookEventRepoIntegrationSuite) TestAnonymize_Idempotent() {
	scenarios := []struct {
		name string
	}{
		{"Anonymize re-executado sobre row já anonimizada é no-op (CA-12 idempotência)"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			event := s.buildWebhookEvent(webhookTestUUID(50), "ext-evt-050")
			_, err := s.webhookRepo.InsertIfNew(s.ctx, event)
			s.Require().NoError(err)

			redacted := json.RawMessage(`{"customer":{"email":"[REDACTED]"}}`)
			now := time.Now().UTC()
			s.Require().NoError(s.webhookRepo.Anonymize(s.ctx, event.ID(), redacted, now))

			redacted2 := json.RawMessage(`{"customer":{"email":"[REDACTED]"}}`)
			err = s.webhookRepo.Anonymize(s.ctx, event.ID(), redacted2, now.Add(time.Second))
			s.NoError(err, "re-execução não deve retornar erro")

			dbtx := s.mgr.DBTX(s.ctx)
			var count int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM webhook_events WHERE id = $1 AND anonymized_at IS NOT NULL",
				event.ID().String()).Scan(&count))
			s.Equal(1, count)
		})
	}
}
