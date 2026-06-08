//go:build integration

package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"
	dbpostgres "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/jobs/handlers"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

const pgImageInteg = "postgres:16"

func setupIntegrationDB(t *testing.T) manager.Manager {
	t.Helper()
	ctx := context.Background()

	req := tc.ContainerRequest{
		Image:        pgImageInteg,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if terr := container.Terminate(context.Background()); terr != nil {
			t.Logf("container terminate: %v", terr)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}

	mapped, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}

	portNum, err := strconv.Atoi(mapped.Port())
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	cfg := dbpostgres.PostgresConfig{
		Host:     host,
		Port:     portNum,
		User:     "test",
		Password: "test",
		Database: "testdb",
		SSLMode:  "disable",
	}

	mgr, err := manager.New(cfg)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	t.Cleanup(func() {
		_ = mgr.Shutdown(context.Background())
	})

	dsn := fmt.Sprintf("pgx5://test:test@%s:%d/testdb?sslmode=disable", host, portNum)
	migrator, err := migration.New(mgr, migration.EmbedFS{FS: migrations.FS, Root: "."}, migration.WithDSN(dsn))
	if err != nil {
		t.Fatalf("failed to create migrator: %v", err)
	}

	if err := migrator.Up(ctx); err != nil && !errors.Is(err, migration.ErrNoChange) {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return mgr
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
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func TestReconciliationIntegration(t *testing.T) {
	suite.Run(t, new(ReconciliationIntegrationSuite))
}

func (s *ReconciliationIntegrationSuite) SetupTest() {}

func (s *ReconciliationIntegrationSuite) SetupSuite() {
	s.mgr = setupIntegrationDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *ReconciliationIntegrationSuite) buildJob(kiwifyMock interfaces.KiwifyClient) *handlers.ReconciliationJob {
	o11y := noop.NewProvider()
	db := s.mgr.DBTX(context.Background())

	publisherMock := ucmocks.NewSubscriptionEventPublisher(s.T())
	publisherMock.EXPECT().PublishActivated(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	publisherMock.EXPECT().PublishRefunded(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	saleApproved := usecases.NewProcessSaleApproved(
		uow.New[entities.Subscription](s.mgr, uow.WithObservability(o11y)),
		s.factory,
		publisherMock,
		o11y,
	)
	refundUC := usecases.NewProcessRefundOrChargeback(
		uow.New[entities.Subscription](s.mgr, uow.WithObservability(o11y)),
		s.factory,
		publisherMock,
		o11y,
	)
	reconcile := usecases.NewReconcileSubscriptions(db, s.factory, kiwifyMock, saleApproved, refundUC, o11y)
	runReconciliation := usecases.NewRunReconciliation(db, s.factory, reconcile, o11y)

	cfg := configs.KiwifyConfig{ReconciliationInterval: "@hourly"}
	return handlers.NewReconciliationJob(runReconciliation, cfg)
}

func (s *ReconciliationIntegrationSuite) insertActiveSubscription(orderID, funnelToken string) {
	ctx := context.Background()
	db := s.mgr.DBTX(ctx)
	plan := makeMonthlyPlan(s.T())
	ft := makeFunnelToken(s.T(), funnelToken)
	now := time.Now().UTC()

	sub := entities.NewSubscription(plan, ft)
	s.Require().NoError(sub.Activate(now))
	subRepo := s.factory.SubscriptionRepository(db)
	s.Require().NoError(subRepo.UpsertByOrder(ctx, orderID, sub, now))
}

func (s *ReconciliationIntegrationSuite) setCheckpoint(name string, watermark time.Time) {
	ctx := context.Background()
	db := s.mgr.DBTX(ctx)
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
				subRepo := s.factory.SubscriptionRepository(s.mgr.DBTX(ctx))
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
				processedRepo := s.factory.ProcessedEventRepository(s.mgr.DBTX(ctx))
				s.Require().NoError(processedRepo.MarkApplied(ctx, "refund:"+saleID, "order_refunded", saleID, refundTime))
				sale := interfaces.KiwifySale{ID: saleID, OrderID: orderID, Status: "refunded", OccurredAt: refundTime, UpdatedAt: refundTime}
				kiwifyMock := ucmocks.NewKiwifyClient(s.T())
				kiwifyMock.EXPECT().ListSalesUpdatedSince(mock.Anything, mock.Anything, mock.Anything, 1).Return(interfaces.KiwifySalePage{Sales: []interfaces.KiwifySale{sale}, HasMore: false}, nil).Once()
				job := s.buildJob(kiwifyMock)
				s.Require().NoError(job.Run(ctx))
				subRepo := s.factory.SubscriptionRepository(s.mgr.DBTX(ctx))
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
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func TestHousekeepingIntegration(t *testing.T) {
	suite.Run(t, new(HousekeepingIntegrationSuite))
}

func (s *HousekeepingIntegrationSuite) SetupTest() {}

func (s *HousekeepingIntegrationSuite) SetupSuite() {
	s.mgr = setupIntegrationDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *HousekeepingIntegrationSuite) persistEvent(envelopeID string, receivedAt time.Time) {
	ctx := context.Background()
	db := s.mgr.DBTX(ctx)
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
	db := s.mgr.DBTX(ctx)
	var count int
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM billing_kiwify_events WHERE envelope_id = $1`,
		envelopeID,
	).Scan(&count)
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
				db := s.mgr.DBTX(ctx)
				now := time.Now().UTC()
				oldID := fmt.Sprintf("hk-old-%d", now.UnixNano())
				recentID := fmt.Sprintf("hk-recent-%d", now.UnixNano())
				s.persistEvent(oldID, now.Add(-91*24*time.Hour))
				s.persistEvent(recentID, now.Add(-time.Hour))
				cfg := configs.BillingConfig{KiwifyEventsRetentionDays: 90, KiwifyEventsHousekeepingSchedule: "@daily", KiwifyEventsHousekeepingBatch: 500}
				job := handlers.NewKiwifyEventsHousekeepingJob(usecases.NewCleanupKiwifyEvents(db, s.factory, cfg, noop.NewProvider()), cfg)
				s.Require().NoError(job.Run(ctx))
				s.Equal(0, s.countEvents(oldID))
				s.Equal(1, s.countEvents(recentID))
			},
		},
		{
			name: "deve preservar linhas dentro da janela de retencao",
			expect: func(ctx context.Context) {
				db := s.mgr.DBTX(ctx)
				now := time.Now().UTC()
				boundaryID := fmt.Sprintf("hk-boundary-%d", now.UnixNano())
				s.persistEvent(boundaryID, now.Add(-89*24*time.Hour))
				cfg := configs.BillingConfig{KiwifyEventsRetentionDays: 90, KiwifyEventsHousekeepingSchedule: "@daily", KiwifyEventsHousekeepingBatch: 500}
				job := handlers.NewKiwifyEventsHousekeepingJob(usecases.NewCleanupKiwifyEvents(db, s.factory, cfg, noop.NewProvider()), cfg)
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

var _ = input.ReconcileSubscriptionsInput{}
