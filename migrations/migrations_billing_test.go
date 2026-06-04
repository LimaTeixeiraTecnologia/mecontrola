//go:build integration

package migrations_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	devkitdb "github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type BillingMigrationSuite struct {
	suite.Suite
	ctx context.Context
	mgr *dbpkg.Manager
	seq int
}

func TestBillingMigrations(t *testing.T) {
	suite.Run(t, new(BillingMigrationSuite))
}

func (s *BillingMigrationSuite) SetupSuite() {
	s.ctx = context.Background()

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
	mappedPort, err := container.MappedPort(s.ctx, "5432")
	s.Require().NoError(err)

	cfg := &configs.Config{
		DBConfig: configs.DBConfig{
			Host:     host,
			Port:     int(mappedPort.Num()),
			User:     "testuser",
			Password: "testpassword",
			Name:     "testdb",
			SSLMode:  "disable",
			MaxConns: 5,
			MinConns: 1,
		},
	}

	mgr, err := dbpkg.NewManager(s.ctx, cfg, nil)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
}

func (s *BillingMigrationSuite) TearDownSuite() {
	if s.mgr != nil {
		_ = s.mgr.Shutdown(context.Background())
	}
}

func (s *BillingMigrationSuite) dbtx() devkitdb.DBTX {
	return s.mgr.DBTX(s.ctx)
}

func (s *BillingMigrationSuite) nextSeq() int {
	s.seq++
	return s.seq
}

func (s *BillingMigrationSuite) queryInt(query string, args ...any) int {
	row := s.dbtx().QueryRowContext(s.ctx, query, args...)
	var n int
	s.Require().NoError(row.Scan(&n))
	return n
}

func (s *BillingMigrationSuite) TestBillingPlansSeedCount() {
	scenarios := []struct {
		name     string
		expected int
	}{
		{name: "deve conter exatamente 3 planos após seed 0010", expected: 3},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			count := s.queryInt("SELECT count(*) FROM billing_plans")
			s.Equal(sc.expected, count)
		})
	}
}

func (s *BillingMigrationSuite) TestBillingPlansIdempotencia() {
	scenarios := []struct {
		name string
	}{
		{name: "re-inserção com ON CONFLICT DO NOTHING deve manter exatamente 3 linhas"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			_, err := s.dbtx().ExecContext(s.ctx, `
				INSERT INTO billing_plans (plan_code, display_name, period_length_days, price_brl_cents)
				VALUES
				    ('MONTHLY',   'Mensal',     30,  2990),
				    ('QUARTERLY', 'Trimestral', 90,  8073),
				    ('ANNUAL',    'Anual',      365, 29780)
				ON CONFLICT (plan_code) DO NOTHING
			`)
			s.NoError(err)

			count := s.queryInt("SELECT count(*) FROM billing_plans")
			s.Equal(3, count)
		})
	}
}

func (s *BillingMigrationSuite) TestBillingPlansValoresCorretos() {
	scenarios := []struct {
		planCode         string
		displayName      string
		periodLengthDays int
		priceBRLCents    int64
	}{
		{"MONTHLY", "Mensal", 30, 2990},
		{"QUARTERLY", "Trimestral", 90, 8073},
		{"ANNUAL", "Anual", 365, 29780},
	}

	for _, sc := range scenarios {
		s.Run(fmt.Sprintf("plano %s deve ter valores corretos", sc.planCode), func() {
			row := s.dbtx().QueryRowContext(s.ctx,
				"SELECT display_name, period_length_days, price_brl_cents FROM billing_plans WHERE plan_code = $1",
				sc.planCode,
			)
			var displayName string
			var periodDays int
			var priceCents int64
			s.Require().NoError(row.Scan(&displayName, &periodDays, &priceCents))
			s.Equal(sc.displayName, displayName)
			s.Equal(sc.periodLengthDays, periodDays)
			s.Equal(sc.priceBRLCents, priceCents)
		})
	}
}

func (s *BillingMigrationSuite) TestWebhookEventsUniqueConstraint() {
	scenarios := []struct {
		name string
	}{
		{name: "inserção duplicada em (provider, external_event_id) deve falhar com UniqueViolation"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			n := s.nextSeq()
			_, err := s.dbtx().ExecContext(s.ctx, `
				INSERT INTO webhook_events (id, provider, external_event_id, event_type, payload)
				VALUES ($1, 'kiwify', $2, 'compra_aprovada', '{}')
			`, fmt.Sprintf("wh-uniq-a-%d", n), fmt.Sprintf("ext-uniq-%d", n))
			s.Require().NoError(err)

			_, err = s.dbtx().ExecContext(s.ctx, `
				INSERT INTO webhook_events (id, provider, external_event_id, event_type, payload)
				VALUES ($1, 'kiwify', $2, 'compra_aprovada', '{}')
			`, fmt.Sprintf("wh-uniq-b-%d", n), fmt.Sprintf("ext-uniq-%d", n))
			s.Require().Error(err)
			s.True(isPgError(err, pgerrcode.UniqueViolation), "esperado UniqueViolation, obtido: %v", err)
		})
	}
}

func (s *BillingMigrationSuite) TestSubscriptionsStatusCheckConstraint() {
	scenarios := []struct {
		name string
	}{
		{name: "INSERT em subscriptions com status='INVALID' deve falhar com CheckViolation"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			n := s.nextSeq()
			userID := s.insertTestUser(n)
			webhookEventID := s.insertTestWebhookEvent(fmt.Sprintf("wh-ck-status-%d", n), fmt.Sprintf("ext-ck-status-%d", n))

			_, err := s.dbtx().ExecContext(s.ctx, `
				INSERT INTO subscriptions
				    (id, user_id, provider, external_subscription_id, plan_code, status,
				     period_start, period_end, last_event_at, last_webhook_event_id)
				VALUES
				    ($1, $2, 'kiwify', $3, 'MONTHLY', 'INVALID',
				     now(), now() + interval '30 days', now(), $4)
			`, fmt.Sprintf("sub-ck-status-%d", n), userID, fmt.Sprintf("ext-sub-ck-status-%d", n), webhookEventID)

			s.Require().Error(err)
			s.True(isPgError(err, pgerrcode.CheckViolation), "esperado CheckViolation, obtido: %v", err)
		})
	}
}

func (s *BillingMigrationSuite) TestSubscriptionsOneActivePerUserConstraint() {
	scenarios := []struct {
		name string
	}{
		{name: "duas subscriptions ACTIVE para o mesmo user_id deve falhar com UniqueViolation"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			n := s.nextSeq()
			userID := s.insertTestUser(n)

			wh1 := s.insertTestWebhookEvent(fmt.Sprintf("wh-oap-1-%d", n), fmt.Sprintf("ext-oap-1-%d", n))
			_, err := s.dbtx().ExecContext(s.ctx, `
				INSERT INTO subscriptions
				    (id, user_id, provider, external_subscription_id, plan_code, status,
				     period_start, period_end, last_event_at, last_webhook_event_id)
				VALUES
				    ($1, $2, 'kiwify', $3, 'MONTHLY', 'ACTIVE',
				     now(), now() + interval '30 days', now(), $4)
			`, fmt.Sprintf("sub-oap-1-%d", n), userID, fmt.Sprintf("ext-sub-oap-1-%d", n), wh1)
			s.Require().NoError(err)

			wh2 := s.insertTestWebhookEvent(fmt.Sprintf("wh-oap-2-%d", n), fmt.Sprintf("ext-oap-2-%d", n))
			_, err = s.dbtx().ExecContext(s.ctx, `
				INSERT INTO subscriptions
				    (id, user_id, provider, external_subscription_id, plan_code, status,
				     period_start, period_end, last_event_at, last_webhook_event_id)
				VALUES
				    ($1, $2, 'kiwify', $3, 'MONTHLY', 'ACTIVE',
				     now(), now() + interval '30 days', now(), $4)
			`, fmt.Sprintf("sub-oap-2-%d", n), userID, fmt.Sprintf("ext-sub-oap-2-%d", n), wh2)
			s.Require().Error(err)
			s.True(isPgError(err, pgerrcode.UniqueViolation), "esperado UniqueViolation, obtido: %v", err)
		})
	}
}

func (s *BillingMigrationSuite) TestSubscriptionsPeriodCheckConstraint() {
	scenarios := []struct {
		name string
	}{
		{name: "period_end anterior a period_start deve falhar com CheckViolation"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			n := s.nextSeq()
			userID := s.insertTestUser(n)
			webhookEventID := s.insertTestWebhookEvent(fmt.Sprintf("wh-period-ck-%d", n), fmt.Sprintf("ext-period-ck-%d", n))

			_, err := s.dbtx().ExecContext(s.ctx, `
				INSERT INTO subscriptions
				    (id, user_id, provider, external_subscription_id, plan_code, status,
				     period_start, period_end, last_event_at, last_webhook_event_id)
				VALUES
				    ($1, $2, 'kiwify', $3, 'MONTHLY', 'ACTIVE',
				     now(), now() - interval '1 second', now(), $4)
			`, fmt.Sprintf("sub-period-ck-%d", n), userID, fmt.Sprintf("ext-sub-period-ck-%d", n), webhookEventID)

			s.Require().Error(err)
			s.True(isPgError(err, pgerrcode.CheckViolation), "esperado CheckViolation, obtido: %v", err)
		})
	}
}

func (s *BillingMigrationSuite) insertTestUser(seq int) string {
	id := fmt.Sprintf("00000000-0000-4000-a000-%012x", seq)
	phone := fmt.Sprintf("+5511900%06d", seq)
	_, err := s.dbtx().ExecContext(s.ctx,
		"INSERT INTO users (id, whatsapp_number, status) VALUES ($1, $2, 'ACTIVE') ON CONFLICT (id) DO NOTHING",
		id, phone,
	)
	s.Require().NoError(err)
	return id
}

func (s *BillingMigrationSuite) insertTestWebhookEvent(id, externalEventID string) string {
	_, err := s.dbtx().ExecContext(s.ctx, `
		INSERT INTO webhook_events (id, provider, external_event_id, event_type, payload)
		VALUES ($1, 'kiwify', $2, 'compra_aprovada', '{}')
		ON CONFLICT (provider, external_event_id) DO NOTHING
	`, id, externalEventID)
	s.Require().NoError(err)
	return id
}

func isPgError(err error, code string) bool {
	var pgErr *pgconn.PgError
	if e, ok := err.(*pgconn.PgError); ok {
		pgErr = e
		return pgErr.Code == code
	}
	return false
}
