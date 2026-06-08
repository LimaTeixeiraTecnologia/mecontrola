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

type ProcessSaleApprovedSuite struct {
	suite.Suite
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	eventRepoMock *mocks.ProcessedEventRepository
	planRepoMock  *mocks.PlanRepository
	publisherMock *mocks.SubscriptionEventPublisher
	uc            *usecases.ProcessSaleApproved
}

func TestProcessSaleApproved(t *testing.T) {
	suite.Run(t, new(ProcessSaleApprovedSuite))
}

func (s *ProcessSaleApprovedSuite) SetupTest() {
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())
	s.planRepoMock = mocks.NewPlanRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
	s.uc = usecases.NewProcessSaleApproved(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
}

func (s *ProcessSaleApprovedSuite) validPlan() valueobjects.Plan {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	return plan
}

func (s *ProcessSaleApprovedSuite) validInput() input.ProcessSaleApprovedInput {
	return input.ProcessSaleApprovedInput{
		EnvelopeID:         "env-001",
		SaleID:             "sale-001",
		KiwifyProductID:    "prod-monthly",
		OrderID:            "order-001",
		FunnelToken:        "token-abc",
		CustomerMobileE164: "+5511999999999",
		CustomerEmail:      "test@example.com",
		OccurredAt:         time.Now().UTC(),
	}
}

func (s *ProcessSaleApprovedSuite) hydrateSub() entities.Subscription {
	plan := s.validPlan()
	ft, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	return entities.Hydrate("sub-001", ft, plan, valueobjects.StatusActive,
		time.Now().UTC(), time.Now().UTC().Add(30*24*time.Hour), time.Time{}, time.Now().UTC())
}

func (s *ProcessSaleApprovedSuite) hydrateSubNoToken() entities.Subscription {
	plan := s.validPlan()
	return entities.Hydrate("sub-002", valueobjects.FunnelToken{}, plan, valueobjects.StatusActive,
		time.Now().UTC(), time.Now().UTC().Add(30*24*time.Hour), time.Time{}, time.Now().UTC())
}

func (s *ProcessSaleApprovedSuite) TestSucessoCriaNovaSub() {
	in := s.validInput()
	sub := s.hydrateSub()

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("PlanRepository", mock.Anything).Return(s.planRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", in.OccurredAt).Return(nil)
	s.planRepoMock.On("FindByKiwifyProductID", mock.Anything, "prod-monthly").Return(s.validPlan(), nil)
	s.subRepoMock.On("UpsertByOrder", mock.Anything, "order-001", mock.Anything, mock.Anything).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil)
	s.publisherMock.On("PublishActivated", mock.Anything, mock.Anything, mock.Anything, "sub-001", "token-abc", "+5511999999999", "test@example.com", "sale-001").Return(nil)

	err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
}

func (s *ProcessSaleApprovedSuite) TestFunnelTokenVazioPublicaWithoutToken() {
	in := s.validInput()
	in.FunnelToken = ""
	sub := s.hydrateSubNoToken()

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("PlanRepository", mock.Anything).Return(s.planRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", in.OccurredAt).Return(nil)
	s.planRepoMock.On("FindByKiwifyProductID", mock.Anything, "prod-monthly").Return(s.validPlan(), nil)
	s.subRepoMock.On("UpsertByOrder", mock.Anything, "order-001", mock.Anything, mock.Anything).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil)
	s.publisherMock.On("PublishActivatedWithoutToken", mock.Anything, mock.Anything, mock.Anything, "sub-002", "+5511999999999", "test@example.com", "sale-001").Return(nil)

	err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
}

func (s *ProcessSaleApprovedSuite) TestPlanoNaoEncontradoRetornaErro() {
	in := s.validInput()

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("PlanRepository", mock.Anything).Return(s.planRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", in.OccurredAt).Return(nil)
	s.planRepoMock.On("FindByKiwifyProductID", mock.Anything, "prod-monthly").Return(valueobjects.Plan{}, errors.New("not found"))

	err := s.uc.Execute(context.Background(), in)
	s.Require().Error(err)
	s.True(errors.Is(err, usecases.ErrPlanNotFound))
}

func (s *ProcessSaleApprovedSuite) TestIdempotenciaRetornaErrEventoJaProcessado() {
	in := s.validInput()

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("PlanRepository", mock.Anything).Return(s.planRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", in.OccurredAt).Return(interfaces.ErrEventAlreadyProcessed)

	err := s.uc.Execute(context.Background(), in)
	s.Require().Error(err)
	s.True(errors.Is(err, usecases.ErrEventAlreadyProcessed))
}

func (s *ProcessSaleApprovedSuite) TestIdempotencia5xRetorna1TransicaoE4ErrAlreadyProcessed() {
	in := s.validInput()
	sub := s.hydrateSub()

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("PlanRepository", mock.Anything).Return(s.planRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", in.OccurredAt).
		Return(nil).Once()
	s.eventRepoMock.On("MarkApplied", mock.Anything, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", in.OccurredAt).
		Return(interfaces.ErrEventAlreadyProcessed).Times(4)

	s.planRepoMock.On("FindByKiwifyProductID", mock.Anything, "prod-monthly").Return(s.validPlan(), nil).Once()
	s.subRepoMock.On("UpsertByOrder", mock.Anything, "order-001", mock.Anything, mock.Anything).Return(nil).Once()
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil).Once()
	s.publisherMock.On("PublishActivated", mock.Anything, mock.Anything, mock.Anything, "sub-001", "token-abc", "+5511999999999", "test@example.com", "sale-001").Return(nil).Once()

	var transitions, duplicates int
	for range 5 {
		err := s.uc.Execute(context.Background(), in)
		if err == nil {
			transitions++
		} else if errors.Is(err, usecases.ErrEventAlreadyProcessed) {
			duplicates++
		}
	}

	s.Equal(1, transitions)
	s.Equal(4, duplicates)
}
