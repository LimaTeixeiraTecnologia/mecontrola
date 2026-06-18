//go:build integration

package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/producers"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type GraceExpirationIntegrationSuite struct {
	suite.Suite
	db *sqlx.DB
}

func TestGraceExpirationIntegration(t *testing.T) {
	suite.Run(t, new(GraceExpirationIntegrationSuite))
}

func (s *GraceExpirationIntegrationSuite) SetupTest() {}

func (s *GraceExpirationIntegrationSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
}

func (s *GraceExpirationIntegrationSuite) buildJob() *handlers.GraceExpirationJob {
	o11y := noop.NewProvider()
	factory := billingrepos.NewRepositoryFactory(o11y)
	outboxFactory := outbox.NewRepositoryFactory(o11y)
	outboxCfg := configs.OutboxConfig{RetryMaxAttempts: 3}
	publisher := producers.NewSubscriptionEventPublisher(outboxFactory, outboxCfg, id.NewUUIDGenerator(), o11y)
	unit := uow.NewUnitOfWork(s.db)
	graceExpired := usecases.NewProcessSubscriptionGraceExpired(unit, s.db, factory, publisher, o11y)
	cfg := configs.BillingConfig{GraceExpirationSchedule: "@every 30m"}
	return handlers.NewGraceExpirationJob(graceExpired, cfg)
}

func (s *GraceExpirationIntegrationSuite) TestGraceExpiration() {
	scenarios := []struct {
		name   string
		expect func(context.Context)
	}{
		{
			name: "batch de expiracao deve transicionar past_due para expired e publicar eventos no outbox",
			expect: func(ctx context.Context) {
				db := s.db
				n := 3
				graceEnd := time.Now().UTC().Add(-time.Hour)
				for range n {
					insertPastDueSubscription(s.T(), db, graceEnd)
				}
				beforePastDue := countSubscriptionsByStatus(s.T(), db, "PAST_DUE")
				s.Require().GreaterOrEqual(beforePastDue, n)
				beforeExpired := countSubscriptionsByStatus(s.T(), db, "EXPIRED")
				beforeOutbox := countOutboxByEventType(s.T(), db, producers.EventTypeSubscriptionExpired)

				job := s.buildJob()
				s.Require().NoError(job.Run(ctx))

				afterPastDue := countSubscriptionsByStatus(s.T(), db, "PAST_DUE")
				afterExpired := countSubscriptionsByStatus(s.T(), db, "EXPIRED")
				afterOutbox := countOutboxByEventType(s.T(), db, producers.EventTypeSubscriptionExpired)

				s.Equal(beforePastDue-n, afterPastDue)
				s.Equal(beforeExpired+n, afterExpired)
				s.Equal(beforeOutbox+n, afterOutbox)
			},
		},
		{
			name: "execucao dupla e idempotente",
			expect: func(ctx context.Context) {
				db := s.db
				graceEnd := time.Now().UTC().Add(-2 * time.Hour)
				insertPastDueSubscription(s.T(), db, graceEnd)

				job := s.buildJob()
				s.Require().NoError(job.Run(ctx))

				afterFirstPastDue := countSubscriptionsByStatus(s.T(), db, "PAST_DUE")
				afterFirstExpired := countSubscriptionsByStatus(s.T(), db, "EXPIRED")
				afterFirstOutbox := countOutboxByEventType(s.T(), db, producers.EventTypeSubscriptionExpired)

				s.Require().NoError(job.Run(ctx))

				s.Equal(afterFirstPastDue, countSubscriptionsByStatus(s.T(), db, "PAST_DUE"))
				s.Equal(afterFirstExpired, countSubscriptionsByStatus(s.T(), db, "EXPIRED"))
				s.Equal(afterFirstOutbox, countOutboxByEventType(s.T(), db, producers.EventTypeSubscriptionExpired))
			},
		},
		{
			name: "nenhuma assinatura past_due elegivel retorna nil sem inserir outbox",
			expect: func(ctx context.Context) {
				db := s.db
				beforeOutbox := countOutboxByEventType(s.T(), db, producers.EventTypeSubscriptionExpired)

				job := s.buildJob()
				s.Require().NoError(job.Run(ctx))

				afterOutbox := countOutboxByEventType(s.T(), db, producers.EventTypeSubscriptionExpired)
				s.Equal(beforeOutbox, afterOutbox)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.expect(context.Background())
		})
	}
}

func insertPastDueSubscription(t *testing.T, db database.DBTX, graceEnd time.Time) uuid.UUID {
	t.Helper()
	subID := uuid.New()
	now := time.Now().UTC()
	periodStart := now.Add(-35 * 24 * time.Hour)
	periodEnd := now.Add(-5 * 24 * time.Hour)
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO billing_subscriptions
		       (id, funnel_token, kiwify_order_id, kiwify_subscription_id,
		        plan_code, status, period_start, period_end,
		        grace_end, last_event_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'MONTHLY', 'PAST_DUE', $5, $6, $7, $8, now(), now())`,
		subID.String(),
		"token-grace-"+subID.String()[:8],
		"order-grace-"+subID.String()[:8],
		"kiwify-sub-"+subID.String()[:8],
		periodStart,
		periodEnd,
		graceEnd,
		now,
	)
	if err != nil {
		t.Fatalf("insertPastDueSubscription: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM billing_subscriptions WHERE id = $1`, subID.String())
	})
	return subID
}

func countSubscriptionsByStatus(t *testing.T, db database.DBTX, status string) int {
	t.Helper()
	var count int
	err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM billing_subscriptions WHERE status = $1`, status).Scan(&count)
	if err != nil {
		t.Fatalf("countSubscriptionsByStatus(%s): %v", status, err)
	}
	return count
}

func countOutboxByEventType(t *testing.T, db database.DBTX, eventType string) int {
	t.Helper()
	var count int
	err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`, eventType).Scan(&count)
	if err != nil {
		t.Fatalf("countOutboxByEventType(%s): %v", eventType, err)
	}
	return count
}
