package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type ReconcileSubscriptionsSuite struct {
	suite.Suite
	factoryMock      *mocks.RepositoryFactory
	checkpointMock   *mocks.ReconciliationCheckpointRepository
	kiwifyClientMock *mocks.KiwifyClient
	uowMock          *mocks.UnitOfWorkSubscription
	publisherMock    *mocks.SubscriptionEventPublisher
	subRepoMock      *mocks.SubscriptionRepository
	planRepoMock     *mocks.PlanRepository
	eventRepoMock    *mocks.ProcessedEventRepository
	uc               *usecases.ReconcileSubscriptions
}

func TestReconcileSubscriptions(t *testing.T) {
	suite.Run(t, new(ReconcileSubscriptionsSuite))
}

func (s *ReconcileSubscriptionsSuite) SetupTest() {
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.checkpointMock = mocks.NewReconciliationCheckpointRepository(s.T())
	s.kiwifyClientMock = mocks.NewKiwifyClient(s.T())
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.planRepoMock = mocks.NewPlanRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())

	saleApproved := usecases.NewProcessSaleApproved(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
	refund := usecases.NewProcessRefundOrChargeback(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())

	s.uc = usecases.NewReconcileSubscriptions(nil, s.factoryMock, s.kiwifyClientMock, saleApproved, refund, noop.NewProvider())
}

func (s *ReconcileSubscriptionsSuite) monthlyPlan() valueobjects.Plan {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	return plan
}

func (s *ReconcileSubscriptionsSuite) activeSubWithID(id, token string) entities.Subscription {
	plan := s.monthlyPlan()
	ft, err := valueobjects.NewFunnelToken(token)
	s.Require().NoError(err)
	now := time.Now().UTC()
	return entities.Hydrate(id, ft, plan, valueobjects.StatusActive,
		now, now.Add(30*24*time.Hour), time.Time{}, now)
}

func (s *ReconcileSubscriptionsSuite) TestSucessoComSaleApproved() {
	now := time.Now().UTC()
	windowStart := now.Add(-1 * time.Hour)
	windowEnd := now
	saleTime := now.Add(-30 * time.Minute)

	sale := interfaces.KiwifySale{
		ID:              "sale-001",
		KiwifyProductID: "prod-monthly",
		OrderID:         "order-001",
		FunnelToken:     "token-abc",
		Status:          "paid",
		OccurredAt:      saleTime,
		UpdatedAt:       saleTime,
	}

	s.kiwifyClientMock.On("ListSalesUpdatedSince", mock.Anything, windowStart, windowEnd, 1).
		Return(interfaces.KiwifySalePage{Sales: []interfaces.KiwifySale{sale}, HasMore: false}, nil)

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("PlanRepository", mock.Anything).Return(s.planRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)
	s.factoryMock.On("ReconciliationCheckpointRepository", mock.Anything).Return(s.checkpointMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", saleTime).Return(nil)
	s.planRepoMock.On("FindByKiwifyProductID", mock.Anything, "prod-monthly").Return(s.monthlyPlan(), nil)
	s.subRepoMock.On("UpsertByOrder", mock.Anything, "order-001", mock.Anything, mock.Anything).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(s.activeSubWithID("sub-001", "token-abc"), nil)
	s.publisherMock.On("PublishActivated", mock.Anything, mock.Anything, mock.Anything, "sub-001", "token-abc").Return(nil)
	s.checkpointMock.On("Set", mock.Anything, "kiwify_sales", windowEnd).Return(nil)

	err := s.uc.Execute(context.Background(), input.ReconcileSubscriptionsInput{
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	})
	s.Require().NoError(err)
}

func (s *ReconcileSubscriptionsSuite) TestCheckpointNaoAtualizadoEmFalhaDeListagem() {
	now := time.Now().UTC()
	windowStart := now.Add(-1 * time.Hour)
	windowEnd := now

	s.kiwifyClientMock.On("ListSalesUpdatedSince", mock.Anything, windowStart, windowEnd, 1).
		Return(interfaces.KiwifySalePage{}, errors.New("kiwify timeout"))

	err := s.uc.Execute(context.Background(), input.ReconcileSubscriptionsInput{
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	})
	s.Require().Error(err)
	s.checkpointMock.AssertNotCalled(s.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ReconcileSubscriptionsSuite) TestCheckpointNaoAtualizadoEmFalhaDeVenda() {
	now := time.Now().UTC()
	windowStart := now.Add(-time.Hour)
	windowEnd := now
	sale := interfaces.KiwifySale{
		ID:              "sale-failed",
		KiwifyProductID: "missing-plan",
		OrderID:         "order-failed",
		FunnelToken:     "token-failed",
		Status:          "paid",
		OccurredAt:      now.Add(-time.Minute),
	}

	s.kiwifyClientMock.On("ListSalesUpdatedSince", mock.Anything, windowStart, windowEnd, 1).
		Return(interfaces.KiwifySalePage{Sales: []interfaces.KiwifySale{sale}}, nil)
	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("PlanRepository", mock.Anything).Return(s.planRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)
	s.eventRepoMock.On("MarkApplied", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.planRepoMock.On("FindByKiwifyProductID", mock.Anything, "missing-plan").Return(valueobjects.Plan{}, errors.New("not found"))

	err := s.uc.Execute(context.Background(), input.ReconcileSubscriptionsInput{
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	})
	s.Require().Error(err)
	s.checkpointMock.AssertNotCalled(s.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ReconcileSubscriptionsSuite) TestSaleRefundadaRotaParaRefund() {
	now := time.Now().UTC()
	windowStart := now.Add(-1 * time.Hour)
	windowEnd := now
	saleTime := now.Add(-30 * time.Minute)

	sale := interfaces.KiwifySale{
		ID:         "sale-002",
		OrderID:    "order-002",
		Status:     "refunded",
		OccurredAt: saleTime,
		UpdatedAt:  saleTime,
	}
	sub := s.activeSubWithID("sub-002", "token-xyz")

	s.kiwifyClientMock.On("ListSalesUpdatedSince", mock.Anything, windowStart, windowEnd, 1).
		Return(interfaces.KiwifySalePage{Sales: []interfaces.KiwifySale{sale}, HasMore: false}, nil)

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)
	s.factoryMock.On("ReconciliationCheckpointRepository", mock.Anything).Return(s.checkpointMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "refund:sale-002", "compra_reembolsada", "sale-002", saleTime).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-002").Return(sub, nil)
	s.subRepoMock.On("ApplyTransition", mock.Anything, "sub-002", valueobjects.StatusRefunded, time.Time{}, saleTime).Return(nil)
	s.publisherMock.On("PublishRefunded", mock.Anything, mock.Anything, mock.Anything, "sub-002").Return(nil)
	s.checkpointMock.On("Set", mock.Anything, "kiwify_sales", windowEnd).Return(nil)

	err := s.uc.Execute(context.Background(), input.ReconcileSubscriptionsInput{
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	})
	s.Require().NoError(err)
}
