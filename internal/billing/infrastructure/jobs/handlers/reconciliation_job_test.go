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
	job            *handlers.ReconciliationJob
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

	saleApproved := usecases.NewProcessSaleApproved(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
	refund := usecases.NewProcessRefundOrChargeback(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
	reconcile := usecases.NewReconcileSubscriptions(nil, s.factoryMock, s.kiwifyMock, saleApproved, refund, noop.NewProvider())
	runReconciliation := usecases.NewRunReconciliation(nil, s.factoryMock, reconcile, noop.NewProvider())

	cfg := configs.KiwifyConfig{ReconciliationInterval: "@hourly"}
	s.job = handlers.NewReconciliationJob(runReconciliation, cfg)
}

func (s *ReconciliationJobSuite) TestName() {
	s.Equal("billing-reconciliation", s.job.Name())
}

func (s *ReconciliationJobSuite) TestSchedule() {
	s.Equal("@hourly", s.job.Schedule())
}

func (s *ReconciliationJobSuite) TestRunUsesCheckpointAsWindowStart() {
	ctx := context.Background()
	now := time.Now().UTC()
	checkpoint := now.Add(-2 * time.Hour)

	s.factoryMock.On("ReconciliationCheckpointRepository", mock.Anything).Return(s.checkpointMock)
	s.checkpointMock.On("Get", mock.Anything, "kiwify_sales").Return(checkpoint, nil)

	expectedWindowStart := checkpoint.Add(-15 * time.Minute)

	s.kiwifyMock.On("ListSalesUpdatedSince", mock.Anything,
		mock.MatchedBy(func(ws time.Time) bool {
			return ws.After(expectedWindowStart.Add(-time.Second)) && ws.Before(expectedWindowStart.Add(time.Second))
		}),
		mock.Anything, 1).
		Return(interfaces.KiwifySalePage{Sales: nil, HasMore: false}, nil)

	s.checkpointMock.On("Set", mock.Anything, "kiwify_sales", mock.Anything).Return(nil)

	err := s.job.Run(ctx)
	s.Require().NoError(err)
}

func (s *ReconciliationJobSuite) TestRunUsesDefaultLookbackWhenCheckpointMissing() {
	ctx := context.Background()

	s.factoryMock.On("ReconciliationCheckpointRepository", mock.Anything).Return(s.checkpointMock)
	s.checkpointMock.On("Get", mock.Anything, "kiwify_sales").Return(time.Time{}, application.ErrCheckpointNotFound)

	s.kiwifyMock.On("ListSalesUpdatedSince", mock.Anything, mock.Anything, mock.Anything, 1).
		Return(interfaces.KiwifySalePage{Sales: nil, HasMore: false}, nil)

	s.checkpointMock.On("Set", mock.Anything, "kiwify_sales", mock.Anything).Return(nil)

	err := s.job.Run(ctx)
	s.Require().NoError(err)
}

func (s *ReconciliationJobSuite) TestCheckpointNotAdvancedOnListError() {
	ctx := context.Background()
	checkpoint := time.Now().UTC().Add(-time.Hour)

	s.factoryMock.On("ReconciliationCheckpointRepository", mock.Anything).Return(s.checkpointMock)
	s.checkpointMock.On("Get", mock.Anything, "kiwify_sales").Return(checkpoint, nil)

	s.kiwifyMock.On("ListSalesUpdatedSince", mock.Anything, mock.Anything, mock.Anything, 1).
		Return(interfaces.KiwifySalePage{}, errors.New("kiwify timeout"))

	err := s.job.Run(ctx)
	s.Require().Error(err)
	s.checkpointMock.AssertNotCalled(s.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ReconciliationJobSuite) TestRunPaginatesUntilNoMore() {
	ctx := context.Background()
	checkpoint := time.Now().UTC().Add(-time.Hour)

	s.factoryMock.On("ReconciliationCheckpointRepository", mock.Anything).Return(s.checkpointMock)
	s.checkpointMock.On("Get", mock.Anything, "kiwify_sales").Return(checkpoint, nil)

	s.kiwifyMock.On("ListSalesUpdatedSince", mock.Anything, mock.Anything, mock.Anything, 1).
		Return(interfaces.KiwifySalePage{Sales: nil, HasMore: true}, nil)
	s.kiwifyMock.On("ListSalesUpdatedSince", mock.Anything, mock.Anything, mock.Anything, 2).
		Return(interfaces.KiwifySalePage{Sales: nil, HasMore: false}, nil)

	s.checkpointMock.On("Set", mock.Anything, "kiwify_sales", mock.Anything).Return(nil)

	err := s.job.Run(ctx)
	s.Require().NoError(err)
	s.kiwifyMock.AssertNumberOfCalls(s.T(), "ListSalesUpdatedSince", 2)
}

func (s *ReconciliationJobSuite) TestRunSaleRefundedAppliesRefund() {
	ctx := context.Background()
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

	s.factoryMock.On("ReconciliationCheckpointRepository", mock.Anything).Return(s.checkpointMock)
	s.checkpointMock.On("Get", mock.Anything, "kiwify_sales").Return(checkpoint, nil)
	s.kiwifyMock.On("ListSalesUpdatedSince", mock.Anything, mock.Anything, mock.Anything, 1).
		Return(interfaces.KiwifySalePage{Sales: []interfaces.KiwifySale{sale}, HasMore: false}, nil)

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)
	s.eventRepoMock.On("MarkApplied", mock.Anything, "refund:sale-refund-01", "compra_reembolsada", "sale-refund-01", saleTime).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-refund-01").Return(sub, nil)
	s.subRepoMock.On("ApplyTransition", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.publisherMock.On("PublishRefunded", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	s.checkpointMock.On("Set", mock.Anything, "kiwify_sales", mock.Anything).Return(nil)

	err := s.job.Run(ctx)
	s.Require().NoError(err)
}

func (s *ReconciliationJobSuite) TestIdempotencyAlreadyProcessedSaleIsNoOp() {
	ctx := context.Background()
	checkpoint := time.Now().UTC().Add(-time.Hour)
	saleTime := time.Now().UTC().Add(-30 * time.Minute)

	sale := interfaces.KiwifySale{
		ID:         "sale-idem-01",
		OrderID:    "order-idem-01",
		Status:     "refunded",
		OccurredAt: saleTime,
		UpdatedAt:  saleTime,
	}

	s.factoryMock.On("ReconciliationCheckpointRepository", mock.Anything).Return(s.checkpointMock)
	s.checkpointMock.On("Get", mock.Anything, "kiwify_sales").Return(checkpoint, nil)
	s.kiwifyMock.On("ListSalesUpdatedSince", mock.Anything, mock.Anything, mock.Anything, 1).
		Return(interfaces.KiwifySalePage{Sales: []interfaces.KiwifySale{sale}, HasMore: false}, nil)

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)
	s.eventRepoMock.On("MarkApplied", mock.Anything, "refund:sale-idem-01", "compra_reembolsada", "sale-idem-01", saleTime).
		Return(interfaces.ErrEventAlreadyProcessed)

	s.checkpointMock.On("Set", mock.Anything, "kiwify_sales", mock.Anything).Return(nil)

	err := s.job.Run(ctx)
	s.Require().NoError(err)
	s.subRepoMock.AssertNotCalled(s.T(), "FindByOrderID", mock.Anything, mock.Anything)
}
