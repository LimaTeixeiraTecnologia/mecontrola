//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories/postgres"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type SubscriptionRepoIntegrationSuite struct {
	suite.Suite
	ctx     context.Context
	mgr     *dbpkg.Manager
	subRepo *billingrepos.PgxSubscriptionRepository
}

func TestSubscriptionRepoIntegration(t *testing.T) {
	suite.Run(t, new(SubscriptionRepoIntegrationSuite))
}

func (s *SubscriptionRepoIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	cfg := s.startPostgres()

	mgr, err := dbpkg.NewManager(s.ctx, cfg, nil)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
	s.subRepo = billingrepos.NewPgxSubscriptionRepository(s.mgr)
}

func (s *SubscriptionRepoIntegrationSuite) TearDownSuite() {
	if s.mgr != nil {
		s.Require().NoError(s.mgr.Shutdown(context.Background()))
	}
}

func (s *SubscriptionRepoIntegrationSuite) SetupTest() {
	dbtx := s.mgr.DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx,
		"TRUNCATE billing_event_applications, webhook_events, subscriptions, outbox_deliveries, outbox_events, users CASCADE")
	s.Require().NoError(err)
}

func (s *SubscriptionRepoIntegrationSuite) startPostgres() *configs.Config {
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

func (s *SubscriptionRepoIntegrationSuite) mustUserID(v string) identityentities.UserID {
	id, err := identityentities.NewUserID(v)
	s.Require().NoError(err)
	return id
}

func (s *SubscriptionRepoIntegrationSuite) mustSubscriptionID(v string) entities.SubscriptionID {
	id, err := entities.NewSubscriptionID(v)
	s.Require().NoError(err)
	return id
}

func (s *SubscriptionRepoIntegrationSuite) mustExternalSubID(v string) valueobjects.ExternalSubscriptionID {
	id, err := valueobjects.NewExternalSubscriptionID(v)
	s.Require().NoError(err)
	return id
}

func (s *SubscriptionRepoIntegrationSuite) insertUserRow(userID, phone string) {
	dbtx := s.mgr.DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx,
		`INSERT INTO users (id, whatsapp_number, display_name, status, created_at, updated_at)
		 VALUES ($1, $2, 'Test User', 'ACTIVE', now(), now())
		 ON CONFLICT (id) DO NOTHING`,
		userID, phone,
	)
	s.Require().NoError(err)
}

func (s *SubscriptionRepoIntegrationSuite) insertWebhookEventRow(webhookID string) valueobjects.WebhookEventID {
	dbtx := s.mgr.DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx,
		`INSERT INTO webhook_events (id, provider, external_event_id, event_type, signature, headers, payload, received_at)
		 VALUES ($1, 'kiwify', $1, 'compra_aprovada', 'tok', '{}', '{}', now())
		 ON CONFLICT DO NOTHING`,
		webhookID,
	)
	s.Require().NoError(err)
	id, err := valueobjects.NewWebhookEventID(webhookID)
	s.Require().NoError(err)
	return id
}

func (s *SubscriptionRepoIntegrationSuite) buildSubscription(
	subID, userID, extSub string,
	status valueobjects.SubscriptionStatus,
	createdAt time.Time,
	lastWebhookEventID valueobjects.WebhookEventID,
) *entities.Subscription {
	sub, err := entities.NewSubscription(entities.NewSubscriptionParams{
		ID:                 s.mustSubscriptionID(subID),
		UserID:             s.mustUserID(userID),
		Provider:           "kiwify",
		ExternalSubID:      s.mustExternalSubID(extSub),
		PlanCode:           valueobjects.PlanCodeMonthly,
		InitialStatus:      status,
		PeriodStart:        createdAt,
		PeriodEnd:          createdAt.Add(30 * 24 * time.Hour),
		LastEventAt:        createdAt,
		LastWebhookEventID: lastWebhookEventID,
		CreatedAt:          createdAt,
	})
	s.Require().NoError(err)
	return sub
}

func (s *SubscriptionRepoIntegrationSuite) TestUpsert_Insert() {
	scenarios := []struct {
		name string
	}{
		{"insert nova subscription persiste e é recuperável"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.insertUserRow("550e8400-e29b-41d4-a716-446655440001", "+5511999990001")
			wevtID := s.insertWebhookEventRow(webhookTestUUID(101))
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			sub := s.buildSubscription(
				"01HX0000000000000000000001",
				"550e8400-e29b-41d4-a716-446655440001",
				"ext-sub-001",
				valueobjects.SubscriptionStatusActive,
				now,
				wevtID,
			)
			s.Require().NoError(s.subRepo.Upsert(s.ctx, sub))

			found, err := s.subRepo.FindActiveByUserID(s.ctx, s.mustUserID("550e8400-e29b-41d4-a716-446655440001"))
			s.Require().NoError(err)
			s.Equal(sub.ID().String(), found.ID().String())
			s.Equal(valueobjects.SubscriptionStatusActive, found.InternalStatus())
		})
	}
}

func (s *SubscriptionRepoIntegrationSuite) TestUpsert_Update() {
	scenarios := []struct {
		name string
	}{
		{"upsert sobre existente atualiza status via ON CONFLICT"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.insertUserRow("550e8400-e29b-41d4-a716-446655440002", "+5511999990002")
			wevtID := s.insertWebhookEventRow(webhookTestUUID(102))
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			sub := s.buildSubscription(
				"01HX0000000000000000000002",
				"550e8400-e29b-41d4-a716-446655440002",
				"ext-sub-002",
				valueobjects.SubscriptionStatusActive,
				now,
				wevtID,
			)
			s.Require().NoError(s.subRepo.Upsert(s.ctx, sub))

			s.Require().NoError(sub.Cancel(time.Now().UTC()))
			s.Require().NoError(s.subRepo.Upsert(s.ctx, sub))

			dbtx := s.mgr.DBTX(s.ctx)
			var status string
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT status FROM subscriptions WHERE id = $1", sub.ID().String()).Scan(&status))
			s.Equal("CANCELED_PENDING", status)
		})
	}
}

func (s *SubscriptionRepoIntegrationSuite) TestFindActiveByUserID_SoftDeleteInvisible() {
	scenarios := []struct {
		name string
	}{
		{"soft-deleted subscription não aparece em FindActiveByUserID"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.insertUserRow("550e8400-e29b-41d4-a716-446655440003", "+5511999990003")
			wevtID := s.insertWebhookEventRow(webhookTestUUID(103))
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			sub := s.buildSubscription(
				"01HX0000000000000000000003",
				"550e8400-e29b-41d4-a716-446655440003",
				"ext-sub-003",
				valueobjects.SubscriptionStatusActive,
				now,
				wevtID,
			)
			s.Require().NoError(s.subRepo.Upsert(s.ctx, sub))

			dbtx := s.mgr.DBTX(s.ctx)
			_, err := dbtx.ExecContext(s.ctx,
				"UPDATE subscriptions SET deleted_at = now() WHERE id = $1", sub.ID().String())
			s.Require().NoError(err)

			_, err = s.subRepo.FindActiveByUserID(s.ctx, s.mustUserID("550e8400-e29b-41d4-a716-446655440003"))
			s.ErrorIs(err, billingrepos.ErrSubscriptionNotFound)
		})
	}
}

func (s *SubscriptionRepoIntegrationSuite) TestFindActiveByUserIDForUpdate_AcquiresLock() {
	scenarios := []struct {
		name string
	}{
		{"FindActiveByUserIDForUpdate dentro de transação adquire FOR UPDATE e retorna subscription"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.insertUserRow("550e8400-e29b-41d4-a716-446655440004", "+5511999990004")
			wevtID := s.insertWebhookEventRow(webhookTestUUID(104))
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			sub := s.buildSubscription(
				"01HX0000000000000000000004",
				"550e8400-e29b-41d4-a716-446655440004",
				"ext-sub-004",
				valueobjects.SubscriptionStatusActive,
				now,
				wevtID,
			)
			s.Require().NoError(s.subRepo.Upsert(s.ctx, sub))

			tx, err := s.mgr.BeginTx(s.ctx, dbpkg.TxOptions{})
			s.Require().NoError(err)
			defer func() { _ = tx.Rollback(s.ctx) }()

			txCtx := dbpkg.WithTx(s.ctx, tx)
			found, err := s.subRepo.FindActiveByUserIDForUpdate(txCtx, s.mustUserID("550e8400-e29b-41d4-a716-446655440004"))
			s.Require().NoError(err)
			s.Equal(sub.ID().String(), found.ID().String())

			s.Require().NoError(tx.Commit(s.ctx))
		})
	}
}

func (s *SubscriptionRepoIntegrationSuite) TestListByStatusInBatch_StablePagination() {
	scenarios := []struct {
		name string
	}{
		{"ListByStatusInBatch retorna paginação estável sem duplicatas entre páginas"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			for i := range 5 {
				phone := "+551199999900" + string(rune('1'+i))
				userUUID := billingIntTestUUID(10 + i)
				s.insertUserRow(userUUID, phone)
				wevtID := s.insertWebhookEventRow(webhookTestUUID(110 + i))
				sub := s.buildSubscription(
					billingIntTestSubID(10+i),
					userUUID,
					"ext-batch-list-"+string(rune('0'+i)),
					valueobjects.SubscriptionStatusActive,
					time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC),
					wevtID,
				)
				s.Require().NoError(s.subRepo.Upsert(s.ctx, sub))
			}

			statuses := []valueobjects.SubscriptionStatus{valueobjects.SubscriptionStatusActive}
			batch1, err := s.subRepo.ListByStatusInBatch(s.ctx, statuses, time.Time{}, entities.SubscriptionID{}, 3)
			s.Require().NoError(err)
			s.Len(batch1, 3)

			last := batch1[len(batch1)-1]
			batch2, err := s.subRepo.ListByStatusInBatch(s.ctx, statuses, last.CreatedAt(), last.ID(), 3)
			s.Require().NoError(err)
			s.GreaterOrEqual(len(batch2), 1)

			seen := make(map[string]bool)
			for _, b := range batch1 {
				seen[b.ID().String()] = true
			}
			for _, b := range batch2 {
				s.False(seen[b.ID().String()], "cursor deve evitar duplicatas entre páginas")
			}
		})
	}
}

func billingIntTestUUID(n int) string {
	return "550e8400-e29b-41d4-a716-4466554400" + billingPad2(n)
}

func billingIntTestSubID(n int) string {
	return "01HX00000000000000000000" + billingPad2(n)
}

func billingPad2(n int) string {
	s := ""
	if n < 10 {
		s = "0"
	}
	for v := n; v > 0; {
		s += string(rune('0' + v%10))
		v /= 10
	}
	if n == 0 {
		return "00"
	}
	return s
}
