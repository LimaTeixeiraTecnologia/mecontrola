//go:build integration

package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/jobs/handlers"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

func setupIntegrationDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, _ := testcontainer.Postgres(t)
	return db
}

func makeMonthlyPlan(t *testing.T) valueobjects.Plan {
	t.Helper()
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	if err != nil {
		t.Fatalf("failed to create monthly plan: %v", err)
	}
	return plan
}

func makeFunnelToken(t *testing.T, raw string) valueobjects.FunnelToken {
	t.Helper()
	ft, err := valueobjects.NewFunnelToken(raw)
	if err != nil {
		t.Fatalf("failed to create funnel token: %v", err)
	}
	return ft
}

type ReconciliationIntegrationSuite struct {
	suite.Suite
	db      *sqlx.DB
	factory interfaces.RepositoryFactory
}

func TestReconciliationIntegration(t *testing.T) {
	suite.Run(t, new(ReconciliationIntegrationSuite))
}

func (s *ReconciliationIntegrationSuite) SetupTest() {}

func (s *ReconciliationIntegrationSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *ReconciliationIntegrationSuite) buildJob(kiwifyMock interfaces.KiwifyClient) *handlers.ReconciliationJob {
	o11y := noop.NewProvider()
	db := s.db

	publisherMock := ucmocks.NewSubscriptionEventPublisher(s.T())
	publisherMock.EXPECT().PublishActivated(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	publisherMock.EXPECT().PublishRefunded(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	saleApproved := usecases.NewProcessSaleApproved(
		uow.NewUnitOfWork(s.db),
		s.factory,
		publisherMock,
		o11y,
	)
	refundUC := usecases.NewProcessRefundOrChargeback(
		uow.NewUnitOfWork(s.db),
		s.factory,
		publisherMock,
		o11y,
	)
	checkpointRepo := s.factory.ReconciliationCheckpointRepository(db)
	reconcile := usecases.NewReconcileSubscriptions(checkpointRepo, kiwifyMock, saleApproved, refundUC, o11y)
	runReconciliation := usecases.NewRunReconciliation(checkpointRepo, reconcile, o11y)

	cfg := configs.KiwifyConfig{ReconciliationInterval: "@hourly"}
	return handlers.NewReconciliationJob(runReconciliation, cfg)
}

func (s *ReconciliationIntegrationSuite) insertActiveSubscription(orderID, funnelToken string) {
	ctx := context.Background()
	db := s.db
	plan := makeMonthlyPlan(s.T())
	ft := makeFunnelToken(s.T(), funnelToken)
	now := time.Now().UTC()

	sub := entities.NewSubscription(plan, ft)
	s.Require().NoError(sub.Activate(now))
	subRepo := s.factory.SubscriptionRepository(db)
	s.Require().NoError(subRepo.UpsertByOrder(ctx, interfaces.UpsertByOrderParams{
		OrderID:      orderID,
		KiwifySubID:  fmt.Sprintf("kiwify-%s", orderID),
		Subscription: sub,
		PeriodStart:  now,
	}))
}

func (s *ReconciliationIntegrationSuite) setCheckpoint(name string, watermark time.Time) {
	ctx := context.Background()
	db := s.db
	repo := s.factory.ReconciliationCheckpointRepository(db)
	s.Require().NoError(repo.Set(ctx, name, watermark))
}

func (s *ReconciliationIntegrationSuite) TestReconcile() {
	scenarios := []struct {
		name   string
		expect func(context.Context)
	}{
		{
			name: "deve transicionar venda refundada para refunded",
			expect: func(ctx context.Context) {
				now := time.Now().UTC()
				orderID := fmt.Sprintf("order-integ-refund-%d", now.UnixNano())
				saleID := fmt.Sprintf("sale-integ-refund-%d", now.UnixNano())
				funnelToken := fmt.Sprintf("token-integ-%d", now.UnixNano())
				s.insertActiveSubscription(orderID, funnelToken)
				s.setCheckpoint("kiwify_sales", now.Add(-time.Hour))
				refundTime := now.Add(-30 * time.Minute)
				sale := interfaces.KiwifySale{ID: saleID, OrderID: orderID, Status: "refunded", OccurredAt: refundTime, UpdatedAt: refundTime}
				kiwifyMock := ucmocks.NewKiwifyClient(s.T())
				kiwifyMock.EXPECT().ListSalesUpdatedSince(mock.Anything, mock.Anything, mock.Anything, 1).Return(interfaces.KiwifySalePage{Sales: []interfaces.KiwifySale{sale}, HasMore: false}, nil).Once()
				job := s.buildJob(kiwifyMock)
				s.Require().NoError(job.Run(ctx))
				subRepo := s.factory.SubscriptionRepository(s.db)
				sub, findErr := subRepo.FindByOrderID(ctx, orderID)
				s.Require().NoError(findErr)
				s.Equal(valueobjects.StatusRefunded, sub.Status())
			},
		},
		{
			name: "deve tratar venda ja processada como no-op",
			expect: func(ctx context.Context) {
				now := time.Now().UTC()
				orderID := fmt.Sprintf("order-integ-noop-%d", now.UnixNano())
				saleID := fmt.Sprintf("sale-integ-noop-%d", now.UnixNano())
				funnelToken := fmt.Sprintf("token-integ-noop-%d", now.UnixNano())
				s.insertActiveSubscription(orderID, funnelToken)
				s.setCheckpoint("kiwify_sales", now.Add(-time.Hour))
				refundTime := now.Add(-30 * time.Minute)
				processedRepo := s.factory.ProcessedEventRepository(s.db)
				s.Require().NoError(processedRepo.MarkApplied(ctx, "order_refunded:"+saleID, "order_refunded", saleID, refundTime))
				sale := interfaces.KiwifySale{ID: saleID, OrderID: orderID, Status: "refunded", OccurredAt: refundTime, UpdatedAt: refundTime}
				kiwifyMock := ucmocks.NewKiwifyClient(s.T())
				kiwifyMock.EXPECT().ListSalesUpdatedSince(mock.Anything, mock.Anything, mock.Anything, 1).Return(interfaces.KiwifySalePage{Sales: []interfaces.KiwifySale{sale}, HasMore: false}, nil).Once()
				job := s.buildJob(kiwifyMock)
				s.Require().NoError(job.Run(ctx))
				subRepo := s.factory.SubscriptionRepository(s.db)
				sub, findErr := subRepo.FindByOrderID(ctx, orderID)
				s.Require().NoError(findErr)
				s.Equal(valueobjects.StatusActive, sub.Status())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.expect(context.Background())
		})
	}
}

type HousekeepingIntegrationSuite struct {
	suite.Suite
	db      *sqlx.DB
	factory interfaces.RepositoryFactory
}

func TestHousekeepingIntegration(t *testing.T) {
	suite.Run(t, new(HousekeepingIntegrationSuite))
}

func (s *HousekeepingIntegrationSuite) SetupTest() {}

func (s *HousekeepingIntegrationSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *HousekeepingIntegrationSuite) persistEvent(envelopeID string, receivedAt time.Time) {
	ctx := context.Background()
	db := s.db
	rawBody, _ := json.Marshal(map[string]any{"trigger": "order_approved"})
	_, err := db.ExecContext(ctx, `
		INSERT INTO billing_kiwify_events (envelope_id, trigger, raw_body, received_at, signature_status)
		VALUES ($1, 'order_approved', $2, $3, 'valid')
		ON CONFLICT (envelope_id) DO NOTHING`,
		envelopeID, rawBody, receivedAt)
	s.Require().NoError(err)
}

func (s *HousekeepingIntegrationSuite) countEvents(envelopeID string) int {
	ctx := context.Background()
	db := s.db
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM billing_kiwify_events WHERE envelope_id = $1`,
		envelopeID,
	).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *HousekeepingIntegrationSuite) TestHousekeeping() {
	scenarios := []struct {
		name   string
		expect func(context.Context)
	}{
		{
			name: "deve remover linhas antigas",
			expect: func(ctx context.Context) {
				db := s.db
				now := time.Now().UTC()
				oldID := fmt.Sprintf("hk-old-%d", now.UnixNano())
				recentID := fmt.Sprintf("hk-recent-%d", now.UnixNano())
				s.persistEvent(oldID, now.Add(-91*24*time.Hour))
				s.persistEvent(recentID, now.Add(-time.Hour))
				cfg := configs.BillingConfig{KiwifyEventsRetentionDays: 90, KiwifyEventsHousekeepingSchedule: "@daily", KiwifyEventsHousekeepingBatch: 500}
				job := handlers.NewKiwifyEventsHousekeepingJob(usecases.NewCleanupKiwifyEvents(s.factory.KiwifyEventRepository(db), cfg, noop.NewProvider()), cfg)
				s.Require().NoError(job.Run(ctx))
				s.Equal(0, s.countEvents(oldID))
				s.Equal(1, s.countEvents(recentID))
			},
		},
		{
			name: "deve preservar linhas dentro da janela de retencao",
			expect: func(ctx context.Context) {
				db := s.db
				now := time.Now().UTC()
				boundaryID := fmt.Sprintf("hk-boundary-%d", now.UnixNano())
				s.persistEvent(boundaryID, now.Add(-89*24*time.Hour))
				cfg := configs.BillingConfig{KiwifyEventsRetentionDays: 90, KiwifyEventsHousekeepingSchedule: "@daily", KiwifyEventsHousekeepingBatch: 500}
				job := handlers.NewKiwifyEventsHousekeepingJob(usecases.NewCleanupKiwifyEvents(s.factory.KiwifyEventRepository(db), cfg, noop.NewProvider()), cfg)
				s.Require().NoError(job.Run(ctx))
				s.Equal(1, s.countEvents(boundaryID))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.expect(context.Background())
		})
	}
}
