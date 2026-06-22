package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type ProcessSaleApprovedSuite struct {
	suite.Suite
	ctx           context.Context
	obs           *fake.Provider
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	eventRepoMock *mocks.ProcessedEventRepository
	planRepoMock  *mocks.PlanRepository
	publisherMock *mocks.SubscriptionEventPublisher
}

func TestProcessSaleApproved(t *testing.T) {
	suite.Run(t, new(ProcessSaleApprovedSuite))
}

func (s *ProcessSaleApprovedSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())
	s.planRepoMock = mocks.NewPlanRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
}

func (s *ProcessSaleApprovedSuite) validPlan() valueobjects.Plan {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	return plan
}

func (s *ProcessSaleApprovedSuite) hydrateSub() entities.Subscription {
	plan := s.validPlan()
	funnelToken, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	now := time.Now().UTC()
	return entities.Hydrate(
		"sub-001",
		funnelToken,
		plan,
		valueobjects.StatusActive,
		now,
		now.Add(30*24*time.Hour),
		time.Time{},
		now,
	)
}

func (s *ProcessSaleApprovedSuite) hydrateSubNoToken() entities.Subscription {
	plan := s.validPlan()
	now := time.Now().UTC()
	return entities.Hydrate(
		"sub-002",
		valueobjects.FunnelToken{},
		plan,
		valueobjects.StatusActive,
		now,
		now.Add(30*24*time.Hour),
		time.Time{},
		now,
	)
}

type saleApprovedDeps struct {
	factoryMock   *mocks.RepositoryFactory
	publisherMock *mocks.SubscriptionEventPublisher
}

func (s *ProcessSaleApprovedSuite) TestExecute() {
	type args struct {
		input      input.ProcessSaleApprovedInput
		executions int
	}

	type result struct {
		err         error
		transitions int
		duplicates  int
	}

	now := time.Now().UTC()

	baseInput := input.ProcessSaleApprovedInput{
		EnvelopeID:         "env-001",
		SaleID:             "sale-001",
		KiwifyProductID:    "prod-monthly",
		OrderID:            "order-001",
		KiwifySubID:        "kiwify-sub-001",
		FunnelToken:        "token-abc",
		CustomerMobileE164: "+5511999999999",
		CustomerEmail:      "test@example.com",
		OccurredAt:         now,
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies saleApprovedDeps
		expect       func(result)
	}{
		{
			name: "deve retornar erro quando kiwify subscription id for invalido",
			args: args{
				input: func() input.ProcessSaleApprovedInput {
					in := baseInput
					in.KiwifySubID = "   "
					return in
				}(),
				executions: 1,
			},
			dependencies: saleApprovedDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock},
			expect: func(result result) {
				s.Error(result.err)
				s.ErrorIs(result.err, ErrKiwifySubscriptionIDInvalid)
				s.Zero(result.transitions)
				s.Zero(result.duplicates)
			},
		},
		{
			name: "deve criar nova assinatura com token",
			args: args{input: baseInput, executions: 1},
			dependencies: func() saleApprovedDeps {
				sub := s.hydrateSub()
				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(1)
				s.factoryMock.EXPECT().PlanRepository(mock.Anything).Return(s.planRepoMock).Times(1)
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_approved:sale-001", "order_approved", "sale-001", now).
					Return(nil).
					Once()
				s.planRepoMock.EXPECT().
					FindByKiwifyProductID(mock.Anything, "prod-monthly").
					Return(s.validPlan(), nil).
					Once()
				s.subRepoMock.EXPECT().
					UpsertByOrder(mock.Anything, mock.Anything).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(mock.Anything, "order-001").
					Return(sub, nil).
					Once()
				s.publisherMock.EXPECT().
					PublishActivated(
						mock.Anything,
						mock.Anything,
						sub,
						"sub-001",
						"token-abc",
						"+5511999999999",
						"test@example.com",
						"sale-001",
					).
					Return(nil).
					Once()
				return saleApprovedDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(result result) {
				s.NoError(result.err)
				s.Equal(1, result.transitions)
				s.Zero(result.duplicates)
			},
		},
		{
			name: "deve publicar ativacao sem token quando funnel token estiver vazio",
			args: args{
				input: func() input.ProcessSaleApprovedInput {
					in := baseInput
					in.FunnelToken = ""
					return in
				}(),
				executions: 1,
			},
			dependencies: func() saleApprovedDeps {
				sub := s.hydrateSubNoToken()
				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(1)
				s.factoryMock.EXPECT().PlanRepository(mock.Anything).Return(s.planRepoMock).Times(1)
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_approved:sale-001", "order_approved", "sale-001", now).
					Return(nil).
					Once()
				s.planRepoMock.EXPECT().
					FindByKiwifyProductID(mock.Anything, "prod-monthly").
					Return(s.validPlan(), nil).
					Once()
				s.subRepoMock.EXPECT().
					UpsertByOrder(mock.Anything, mock.Anything).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(mock.Anything, "order-001").
					Return(sub, nil).
					Once()
				s.publisherMock.EXPECT().
					PublishActivatedWithoutToken(
						mock.Anything,
						mock.Anything,
						sub,
						"sub-002",
						"+5511999999999",
						"test@example.com",
						"sale-001",
					).
					Return(nil).
					Once()
				return saleApprovedDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(result result) {
				s.NoError(result.err)
				s.Equal(1, result.transitions)
				s.Zero(result.duplicates)
			},
		},
		{
			name: "deve retornar erro quando plano nao for encontrado",
			args: args{input: baseInput, executions: 1},
			dependencies: func() saleApprovedDeps {
				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(1)
				s.factoryMock.EXPECT().PlanRepository(mock.Anything).Return(s.planRepoMock).Times(1)
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_approved:sale-001", "order_approved", "sale-001", now).
					Return(nil).
					Once()
				s.planRepoMock.EXPECT().
					FindByKiwifyProductID(mock.Anything, "prod-monthly").
					Return(valueobjects.Plan{}, errors.New("not found")).
					Once()
				return saleApprovedDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(result result) {
				s.Error(result.err)
				s.ErrorIs(result.err, ErrPlanNotFound)
				s.Zero(result.transitions)
				s.Zero(result.duplicates)
			},
		},
		{
			name: "deve retornar erro de evento ja processado em cenario idempotente",
			args: args{input: baseInput, executions: 1},
			dependencies: func() saleApprovedDeps {
				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(1)
				s.factoryMock.EXPECT().PlanRepository(mock.Anything).Return(s.planRepoMock).Times(1)
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_approved:sale-001", "order_approved", "sale-001", now).
					Return(application.ErrEventAlreadyProcessed).
					Once()
				return saleApprovedDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(result result) {
				s.Error(result.err)
				s.ErrorIs(result.err, ErrEventAlreadyProcessed)
				s.Zero(result.transitions)
				s.Zero(result.duplicates)
			},
		},
		{
			name: "deve aplicar uma transicao e rejeitar quatro duplicidades em cinco execucoes",
			args: args{input: baseInput, executions: 5},
			dependencies: func() saleApprovedDeps {
				sub := s.hydrateSub()
				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(5)
				s.factoryMock.EXPECT().PlanRepository(mock.Anything).Return(s.planRepoMock).Times(5)
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(5)
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_approved:sale-001", "order_approved", "sale-001", now).
					Return(nil).
					Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_approved:sale-001", "order_approved", "sale-001", now).
					Return(application.ErrEventAlreadyProcessed).
					Times(4)
				s.planRepoMock.EXPECT().
					FindByKiwifyProductID(mock.Anything, "prod-monthly").
					Return(s.validPlan(), nil).
					Once()
				s.subRepoMock.EXPECT().
					UpsertByOrder(mock.Anything, mock.Anything).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(mock.Anything, "order-001").
					Return(sub, nil).
					Once()
				s.publisherMock.EXPECT().
					PublishActivated(
						mock.Anything,
						mock.Anything,
						sub,
						"sub-001",
						"token-abc",
						"+5511999999999",
						"test@example.com",
						"sale-001",
					).
					Return(nil).
					Once()
				return saleApprovedDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(result result) {
				s.NoError(result.err)
				s.Equal(1, result.transitions)
				s.Equal(4, result.duplicates)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewProcessSaleApproved(s.uowMock, scenario.dependencies.factoryMock, scenario.dependencies.publisherMock, s.obs)

			result := result{}
			executions := scenario.args.executions
			if executions == 0 {
				executions = 1
			}

			for index := 0; index < executions; index++ {
				err := sut.Execute(s.ctx, scenario.args.input)
				if executions == 1 {
					result.err = err
					if err == nil {
						result.transitions = 1
					}
					continue
				}
				if err == nil {
					result.transitions++
					continue
				}
				if errors.Is(err, ErrEventAlreadyProcessed) {
					result.duplicates++
					continue
				}
				result.err = err
				break
			}

			scenario.expect(result)
		})
	}
}
