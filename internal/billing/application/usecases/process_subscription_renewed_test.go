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

type ProcessSubscriptionRenewedSuite struct {
	suite.Suite
	ctx           context.Context
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	eventRepoMock *mocks.ProcessedEventRepository
	planRepoMock  *mocks.PlanRepository
	publisherMock *mocks.SubscriptionEventPublisher
}

func TestProcessSubscriptionRenewed(t *testing.T) {
	suite.Run(t, new(ProcessSubscriptionRenewedSuite))
}

func (s *ProcessSubscriptionRenewedSuite) SetupTest() {
	s.ctx = context.Background()
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())
	s.planRepoMock = mocks.NewPlanRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
}

func (s *ProcessSubscriptionRenewedSuite) activeSub(lastEventAt time.Time) entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	funnelToken, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	return entities.Hydrate(
		"sub-001",
		funnelToken,
		plan,
		valueobjects.StatusActive,
		lastEventAt.Add(-30*24*time.Hour),
		lastEventAt.Add(30*24*time.Hour),
		time.Time{},
		lastEventAt,
	)
}

func (s *ProcessSubscriptionRenewedSuite) pastDueSub(lastEventAt time.Time) entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	funnelToken, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	return entities.Hydrate(
		"sub-001",
		funnelToken,
		plan,
		valueobjects.StatusPastDue,
		lastEventAt.Add(-30*24*time.Hour),
		lastEventAt.Add(24*time.Hour),
		lastEventAt.Add(3*24*time.Hour),
		lastEventAt,
	)
}

func (s *ProcessSubscriptionRenewedSuite) expectRepositories() {
	s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
	s.factoryMock.EXPECT().PlanRepository(mock.Anything).Return(s.planRepoMock).Once()
	s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
}

func (s *ProcessSubscriptionRenewedSuite) TestExecute() {
	type args struct {
		input input.ProcessSubscriptionRenewedInput
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(args)
		expect func(error)
	}{
		{
			name: "deve estender periodo de assinatura ativa",
			args: args{
				input: func() input.ProcessSubscriptionRenewedInput {
					now := time.Now().UTC()
					return input.ProcessSubscriptionRenewedInput{
						OrderID:         "order-001",
						KiwifySubID:     "kiwify-sub-001",
						KiwifyProductID: "prod-monthly",
						OccurredAt:      now,
					}
				}(),
			},
			setup: func(args args) {
				sub := s.activeSub(args.input.OccurredAt.Add(-time.Hour))
				eventKey := "subscription_renewed:kiwify-sub-001:" + args.input.OccurredAt.Format("2006-01-02T15:04:05Z07:00")
				expectedPeriodEnd := sub.PeriodEnd().Add(sub.Plan().Duration())

				s.expectRepositories()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, eventKey, "subscription_renewed", "kiwify-sub-001", args.input.OccurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ExtendPeriod(s.ctx, "sub-001", expectedPeriodEnd, args.input.OccurredAt).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishRenewed(
						s.ctx,
						mock.Anything,
						mock.MatchedBy(func(renewed entities.Subscription) bool {
							return renewed.ID() == "sub-001" &&
								renewed.Status() == valueobjects.StatusActive &&
								renewed.PeriodEnd().Equal(expectedPeriodEnd) &&
								renewed.LastEventAt().Equal(args.input.OccurredAt)
						}),
						"sub-001",
						sub.PeriodEnd(),
					).
					Return(nil).
					Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro de evento ja processado em cenario idempotente",
			args: args{
				input: func() input.ProcessSubscriptionRenewedInput {
					now := time.Now().UTC()
					return input.ProcessSubscriptionRenewedInput{
						OrderID:     "order-001",
						KiwifySubID: "kiwify-sub-001",
						OccurredAt:  now,
					}
				}(),
			},
			setup: func(args args) {
				eventKey := "subscription_renewed:kiwify-sub-001:" + args.input.OccurredAt.Format("2006-01-02T15:04:05Z07:00")

				s.expectRepositories()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, eventKey, "subscription_renewed", "kiwify-sub-001", args.input.OccurredAt).
					Return(application.ErrEventAlreadyProcessed).
					Once()
			},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, usecases.ErrEventAlreadyProcessed)
			},
		},
		{
			name: "deve marcar evento stale como superseded quando houver evento mais recente",
			args: args{
				input: func() input.ProcessSubscriptionRenewedInput {
					now := time.Now().UTC()
					return input.ProcessSubscriptionRenewedInput{
						OrderID:     "order-001",
						KiwifySubID: "kiwify-sub-001",
						OccurredAt:  now,
					}
				}(),
			},
			setup: func(args args) {
				sub := s.pastDueSub(args.input.OccurredAt.Add(2 * time.Hour))
				eventKey := "subscription_renewed:kiwify-sub-001:" + args.input.OccurredAt.Format("2006-01-02T15:04:05Z07:00")

				s.expectRepositories()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, eventKey, "subscription_renewed", "kiwify-sub-001", args.input.OccurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(sub, nil).
					Once()
				s.eventRepoMock.EXPECT().
					MarkSuperseded(s.ctx, eventKey).
					Return(nil).
					Once()
			},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, usecases.ErrEventSuperseded)
			},
		},
		{
			name: "deve criar placeholder quando assinatura ainda nao existir",
			args: args{
				input: func() input.ProcessSubscriptionRenewedInput {
					now := time.Now().UTC()
					return input.ProcessSubscriptionRenewedInput{
						OrderID:         "order-001",
						KiwifySubID:     "kiwify-sub-001",
						KiwifyProductID: "prod-monthly",
						OccurredAt:      now,
					}
				}(),
			},
			setup: func(args args) {
				plan, err := valueobjects.NewPlan("MONTHLY", 30)
				s.Require().NoError(err)
				placeholderSub := entities.Hydrate(
					"sub-placeholder",
					valueobjects.FunnelToken{},
					plan,
					valueobjects.StatusActive,
					args.input.OccurredAt,
					args.input.OccurredAt.Add(30*24*time.Hour),
					time.Time{},
					args.input.OccurredAt,
				)
				eventKey := "subscription_renewed:kiwify-sub-001:" + args.input.OccurredAt.Format("2006-01-02T15:04:05Z07:00")

				s.expectRepositories()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, eventKey, "subscription_renewed", "kiwify-sub-001", args.input.OccurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(entities.Subscription{}, errors.New("not found")).
					Once()
				s.planRepoMock.EXPECT().
					FindByKiwifyProductID(s.ctx, "prod-monthly").
					Return(plan, nil).
					Once()
				s.subRepoMock.EXPECT().
					UpsertByOrder(s.ctx, "order-001", mock.Anything, args.input.OccurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(placeholderSub, nil).
					Once()
				s.publisherMock.EXPECT().
					PublishRenewed(s.ctx, mock.Anything, placeholderSub, "sub-placeholder", args.input.OccurredAt).
					Return(nil).
					Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			sut := usecases.NewProcessSubscriptionRenewed(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
			scenario.setup(scenario.args)

			err := sut.Execute(s.ctx, scenario.args.input)

			scenario.expect(err)
		})
	}
}
