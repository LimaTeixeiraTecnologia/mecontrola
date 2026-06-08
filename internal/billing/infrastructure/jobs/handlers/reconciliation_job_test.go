package handlers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/jobs/handlers"
)

func makeActiveSub(s *ReconciliationJobSuite, id, token string) entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	ft, err := valueobjects.NewFunnelToken(token)
	s.Require().NoError(err)
	now := time.Now().UTC()
	return entities.Hydrate(id, ft, plan, valueobjects.StatusActive,
		now, now.Add(30*24*time.Hour), time.Time{}, now)
}

type ReconciliationJobSuite struct {
	suite.Suite
	factoryMock    *ucmocks.RepositoryFactory
	checkpointMock *ucmocks.ReconciliationCheckpointRepository
	kiwifyMock     *ucmocks.KiwifyClient
	uowMock        *ucmocks.UnitOfWorkSubscription
	publisherMock  *ucmocks.SubscriptionEventPublisher
	subRepoMock    *ucmocks.SubscriptionRepository
	planRepoMock   *ucmocks.PlanRepository
	eventRepoMock  *ucmocks.ProcessedEventRepository
}

func TestReconciliationJob(t *testing.T) {
	suite.Run(t, new(ReconciliationJobSuite))
}

func (s *ReconciliationJobSuite) SetupTest() {
	s.factoryMock = ucmocks.NewRepositoryFactory(s.T())
	s.checkpointMock = ucmocks.NewReconciliationCheckpointRepository(s.T())
	s.kiwifyMock = ucmocks.NewKiwifyClient(s.T())
	s.uowMock = ucmocks.NewUnitOfWorkSubscription(s.T())
	s.publisherMock = ucmocks.NewSubscriptionEventPublisher(s.T())
	s.subRepoMock = ucmocks.NewSubscriptionRepository(s.T())
	s.planRepoMock = ucmocks.NewPlanRepository(s.T())
	s.eventRepoMock = ucmocks.NewProcessedEventRepository(s.T())
}

func (s *ReconciliationJobSuite) newJob() *handlers.ReconciliationJob {
	saleApproved := usecases.NewProcessSaleApproved(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
	refund := usecases.NewProcessRefundOrChargeback(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
	reconcile := usecases.NewReconcileSubscriptions(nil, s.factoryMock, s.kiwifyMock, saleApproved, refund, noop.NewProvider())
	runReconciliation := usecases.NewRunReconciliation(nil, s.factoryMock, reconcile, noop.NewProvider())
	return handlers.NewReconciliationJob(runReconciliation, configs.KiwifyConfig{ReconciliationInterval: "@hourly"})
}

func (s *ReconciliationJobSuite) TestMetadata() {
	scenarios := []struct {
		name             string
		expectedName     string
		expectedSchedule string
	}{
		{
			name:             "deve expor o nome e schedule do job",
			expectedName:     "billing-reconciliation",
			expectedSchedule: "@hourly",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			job := s.newJob()
			s.Equal(scenario.expectedName, job.Name())
			s.Equal(scenario.expectedSchedule, job.Schedule())
		})
	}
}

func (s *ReconciliationJobSuite) TestRun() {
	scenarios := []struct {
		name   string
		setup  func(context.Context)
		expect func(error)
	}{
		{
			name: "deve usar o checkpoint como inicio da janela",
			setup: func(ctx context.Context) {
				now := time.Now().UTC()
				checkpoint := now.Add(-2 * time.Hour)
				expectedWindowStart := checkpoint.Add(-15 * time.Minute)

				s.factoryMock.EXPECT().ReconciliationCheckpointRepository(nil).Return(s.checkpointMock).Twice()
				s.checkpointMock.EXPECT().Get(ctx, "kiwify_sales").Return(checkpoint, nil).Once()
				s.kiwifyMock.EXPECT().
					ListSalesUpdatedSince(ctx, mock.MatchedBy(func(ws time.Time) bool {
						return ws.After(expectedWindowStart.Add(-time.Second)) && ws.Before(expectedWindowStart.Add(time.Second))
					}), mock.Anything, 1).
					Return(interfaces.KiwifySalePage{Sales: nil, HasMore: false}, nil).
					Once()
				s.checkpointMock.EXPECT().Set(ctx, "kiwify_sales", mock.Anything).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve usar lookback padrao quando o checkpoint nao existir",
			setup: func(ctx context.Context) {
				s.factoryMock.EXPECT().ReconciliationCheckpointRepository(nil).Return(s.checkpointMock).Twice()
				s.checkpointMock.EXPECT().Get(ctx, "kiwify_sales").Return(time.Time{}, application.ErrCheckpointNotFound).Once()
				s.kiwifyMock.EXPECT().ListSalesUpdatedSince(ctx, mock.Anything, mock.Anything, 1).
					Return(interfaces.KiwifySalePage{Sales: nil, HasMore: false}, nil).
					Once()
				s.checkpointMock.EXPECT().Set(ctx, "kiwify_sales", mock.Anything).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "nao deve avancar o checkpoint quando listar vendas falhar",
			setup: func(ctx context.Context) {
				checkpoint := time.Now().UTC().Add(-time.Hour)
				s.factoryMock.EXPECT().ReconciliationCheckpointRepository(nil).Return(s.checkpointMock).Once()
				s.checkpointMock.EXPECT().Get(ctx, "kiwify_sales").Return(checkpoint, nil).Once()
				s.kiwifyMock.EXPECT().ListSalesUpdatedSince(ctx, mock.Anything, mock.Anything, 1).
					Return(interfaces.KiwifySalePage{}, errors.New("kiwify timeout")).
					Once()
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.checkpointMock.AssertNotCalled(s.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "deve paginar ate nao haver mais paginas",
			setup: func(ctx context.Context) {
				checkpoint := time.Now().UTC().Add(-time.Hour)
				s.factoryMock.EXPECT().ReconciliationCheckpointRepository(nil).Return(s.checkpointMock).Twice()
				s.checkpointMock.EXPECT().Get(ctx, "kiwify_sales").Return(checkpoint, nil).Once()
				s.kiwifyMock.EXPECT().ListSalesUpdatedSince(ctx, mock.Anything, mock.Anything, 1).
					Return(interfaces.KiwifySalePage{Sales: nil, HasMore: true}, nil).
					Once()
				s.kiwifyMock.EXPECT().ListSalesUpdatedSince(ctx, mock.Anything, mock.Anything, 2).
					Return(interfaces.KiwifySalePage{Sales: nil, HasMore: false}, nil).
					Once()
				s.checkpointMock.EXPECT().Set(ctx, "kiwify_sales", mock.Anything).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve aplicar reembolso para venda refundada",
			setup: func(ctx context.Context) {
				now := time.Now().UTC()
				checkpoint := now.Add(-time.Hour)
				saleTime := now.Add(-30 * time.Minute)
				sale := interfaces.KiwifySale{
					ID:         "sale-refund-01",
					OrderID:    "order-refund-01",
					Status:     "refunded",
					OccurredAt: saleTime,
					UpdatedAt:  saleTime,
				}
				sub := makeActiveSub(s, "sub-r01", "token-r01")

				s.factoryMock.EXPECT().ReconciliationCheckpointRepository(nil).Return(s.checkpointMock).Twice()
				s.checkpointMock.EXPECT().Get(ctx, "kiwify_sales").Return(checkpoint, nil).Once()
				s.kiwifyMock.EXPECT().ListSalesUpdatedSince(ctx, mock.Anything, mock.Anything, 1).
					Return(interfaces.KiwifySalePage{Sales: []interfaces.KiwifySale{sale}, HasMore: false}, nil).
					Once()
				s.factoryMock.EXPECT().ProcessedEventRepository(nil).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(nil).Return(s.subRepoMock).Once()
				s.eventRepoMock.EXPECT().MarkApplied(ctx, "refund:sale-refund-01", "compra_reembolsada", "sale-refund-01", saleTime).Return(nil).Once()
				s.subRepoMock.EXPECT().FindByOrderID(ctx, "order-refund-01").Return(sub, nil).Once()
				s.subRepoMock.EXPECT().ApplyTransition(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				s.publisherMock.EXPECT().PublishRefunded(ctx, nil, mock.Anything, mock.Anything).Return(nil).Once()
				s.checkpointMock.EXPECT().Set(ctx, "kiwify_sales", mock.Anything).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve tratar venda ja processada como no-op",
			setup: func(ctx context.Context) {
				checkpoint := time.Now().UTC().Add(-time.Hour)
				saleTime := time.Now().UTC().Add(-30 * time.Minute)
				sale := interfaces.KiwifySale{
					ID:         "sale-idem-01",
					OrderID:    "order-idem-01",
					Status:     "refunded",
					OccurredAt: saleTime,
					UpdatedAt:  saleTime,
				}

				s.factoryMock.EXPECT().ReconciliationCheckpointRepository(nil).Return(s.checkpointMock).Twice()
				s.checkpointMock.EXPECT().Get(ctx, "kiwify_sales").Return(checkpoint, nil).Once()
				s.kiwifyMock.EXPECT().ListSalesUpdatedSince(ctx, mock.Anything, mock.Anything, 1).
					Return(interfaces.KiwifySalePage{Sales: []interfaces.KiwifySale{sale}, HasMore: false}, nil).
					Once()
				s.factoryMock.EXPECT().ProcessedEventRepository(nil).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(nil).Return(s.subRepoMock).Once()
				s.eventRepoMock.EXPECT().MarkApplied(ctx, "refund:sale-idem-01", "compra_reembolsada", "sale-idem-01", saleTime).
					Return(interfaces.ErrEventAlreadyProcessed).
					Once()
				s.checkpointMock.EXPECT().Set(ctx, "kiwify_sales", mock.Anything).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
				s.subRepoMock.AssertNotCalled(s.T(), "FindByOrderID", mock.Anything, mock.Anything)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			ctx := context.Background()
			scenario.setup(ctx)
			job := s.newJob()
			err := job.Run(ctx)
			scenario.expect(err)
		})
	}
}
