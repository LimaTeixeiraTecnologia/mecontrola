package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type ProcessSaleApprovedSuite struct {
	suite.Suite
	ctx           context.Context
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

func (s *ProcessSaleApprovedSuite) expectRepositories(times int) {
	s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(times)
	s.factoryMock.EXPECT().PlanRepository(mock.Anything).Return(s.planRepoMock).Times(times)
	s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(times)
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

	scenarios := []struct {
		name   string
		args   args
		setup  func(args)
		expect func(result)
	}{
		{
			name: "deve criar nova assinatura com token",
			args: args{input: s.validInput(), executions: 1},
			setup: func(args args) {
				sub := s.hydrateSub()
				s.expectRepositories(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", args.input.OccurredAt).
					Return(nil).
					Once()
				s.planRepoMock.EXPECT().
					FindByKiwifyProductID(s.ctx, "prod-monthly").
					Return(s.validPlan(), nil).
					Once()
				s.subRepoMock.EXPECT().
					UpsertByOrder(s.ctx, "order-001", mock.Anything, args.input.OccurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(sub, nil).
					Once()
				s.publisherMock.EXPECT().
					PublishActivated(
						s.ctx,
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
			},
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
					in := s.validInput()
					in.FunnelToken = ""
					return in
				}(),
				executions: 1,
			},
			setup: func(args args) {
				sub := s.hydrateSubNoToken()
				s.expectRepositories(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", args.input.OccurredAt).
					Return(nil).
					Once()
				s.planRepoMock.EXPECT().
					FindByKiwifyProductID(s.ctx, "prod-monthly").
					Return(s.validPlan(), nil).
					Once()
				s.subRepoMock.EXPECT().
					UpsertByOrder(s.ctx, "order-001", mock.Anything, args.input.OccurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(sub, nil).
					Once()
				s.publisherMock.EXPECT().
					PublishActivatedWithoutToken(
						s.ctx,
						mock.Anything,
						sub,
						"sub-002",
						"+5511999999999",
						"test@example.com",
						"sale-001",
					).
					Return(nil).
					Once()
			},
			expect: func(result result) {
				s.NoError(result.err)
				s.Equal(1, result.transitions)
				s.Zero(result.duplicates)
			},
		},
		{
			name: "deve retornar erro quando plano nao for encontrado",
			args: args{input: s.validInput(), executions: 1},
			setup: func(args args) {
				s.expectRepositories(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", args.input.OccurredAt).
					Return(nil).
					Once()
				s.planRepoMock.EXPECT().
					FindByKiwifyProductID(s.ctx, "prod-monthly").
					Return(valueobjects.Plan{}, errors.New("not found")).
					Once()
			},
			expect: func(result result) {
				s.Error(result.err)
				s.ErrorIs(result.err, usecases.ErrPlanNotFound)
				s.Zero(result.transitions)
				s.Zero(result.duplicates)
			},
		},
		{
			name: "deve retornar erro de evento ja processado em cenario idempotente",
			args: args{input: s.validInput(), executions: 1},
			setup: func(args args) {
				s.expectRepositories(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", args.input.OccurredAt).
					Return(application.ErrEventAlreadyProcessed).
					Once()
			},
			expect: func(result result) {
				s.Error(result.err)
				s.ErrorIs(result.err, usecases.ErrEventAlreadyProcessed)
				s.Zero(result.transitions)
				s.Zero(result.duplicates)
			},
		},
		{
			name: "deve aplicar uma transicao e rejeitar quatro duplicidades em cinco execucoes",
			args: args{input: s.validInput(), executions: 5},
			setup: func(args args) {
				sub := s.hydrateSub()
				s.expectRepositories(args.executions)
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", args.input.OccurredAt).
					Return(nil).
					Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", args.input.OccurredAt).
					Return(application.ErrEventAlreadyProcessed).
					Times(4)
				s.planRepoMock.EXPECT().
					FindByKiwifyProductID(s.ctx, "prod-monthly").
					Return(s.validPlan(), nil).
					Once()
				s.subRepoMock.EXPECT().
					UpsertByOrder(s.ctx, "order-001", mock.Anything, args.input.OccurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(sub, nil).
					Once()
				s.publisherMock.EXPECT().
					PublishActivated(
						s.ctx,
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
			},
			expect: func(result result) {
				s.NoError(result.err)
				s.Equal(1, result.transitions)
				s.Equal(4, result.duplicates)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			sut := usecases.NewProcessSaleApproved(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
			scenario.setup(scenario.args)

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
				if errors.Is(err, usecases.ErrEventAlreadyProcessed) {
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
