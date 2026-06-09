package usecases_test

import (
	"context"
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

type ProcessSubscriptionCanceledSuite struct {
	suite.Suite
	ctx           context.Context
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	eventRepoMock *mocks.ProcessedEventRepository
	publisherMock *mocks.SubscriptionEventPublisher
}

func TestProcessSubscriptionCanceled(t *testing.T) {
	suite.Run(t, new(ProcessSubscriptionCanceledSuite))
}

func (s *ProcessSubscriptionCanceledSuite) SetupTest() {
	s.ctx = context.Background()
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
}

func (s *ProcessSubscriptionCanceledSuite) activeSub(lastEventAt time.Time) entities.Subscription {
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

func (s *ProcessSubscriptionCanceledSuite) expectRepositories() {
	s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
	s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
}

func (s *ProcessSubscriptionCanceledSuite) TestExecute() {
	type args struct {
		input input.ProcessSubscriptionCanceledInput
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(args)
		expect func(error)
	}{
		{
			name: "deve transicionar para canceled pending preservando period end e grace end zerado",
			args: args{
				input: func() input.ProcessSubscriptionCanceledInput {
					now := time.Now().UTC()
					return input.ProcessSubscriptionCanceledInput{
						OrderID:     "order-001",
						KiwifySubID: "kiwify-sub-001",
						OccurredAt:  now,
					}
				}(),
			},
			setup: func(args args) {
				sub := s.activeSub(args.input.OccurredAt.Add(-2 * time.Hour))
				eventKey := "subscription_canceled:kiwify-sub-001"

				s.expectRepositories()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, eventKey, "subscription_canceled", "kiwify-sub-001", args.input.OccurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByKiwifySubID(s.ctx, "kiwify-sub-001").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ApplyTransition(s.ctx, "sub-001", valueobjects.StatusCanceledPending, time.Time{}, args.input.OccurredAt).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishCanceled(
						s.ctx,
						mock.Anything,
						mock.MatchedBy(func(updated entities.Subscription) bool {
							return updated.ID() == "sub-001" &&
								updated.Status() == valueobjects.StatusCanceledPending &&
								updated.PeriodEnd().Equal(sub.PeriodEnd()) &&
								updated.GraceEnd().IsZero() &&
								updated.LastEventAt().Equal(args.input.OccurredAt)
						}),
						"sub-001",
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
				input: func() input.ProcessSubscriptionCanceledInput {
					now := time.Now().UTC()
					return input.ProcessSubscriptionCanceledInput{
						OrderID:     "order-001",
						KiwifySubID: "kiwify-sub-001",
						OccurredAt:  now,
					}
				}(),
			},
			setup: func(args args) {
				eventKey := "subscription_canceled:kiwify-sub-001"

				s.expectRepositories()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, eventKey, "subscription_canceled", "kiwify-sub-001", args.input.OccurredAt).
					Return(application.ErrEventAlreadyProcessed).
					Once()
			},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, usecases.ErrEventAlreadyProcessed)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			sut := usecases.NewProcessSubscriptionCanceled(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
			scenario.setup(scenario.args)

			err := sut.Execute(s.ctx, scenario.args.input)

			scenario.expect(err)
		})
	}
}
