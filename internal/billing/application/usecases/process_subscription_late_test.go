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

type ProcessSubscriptionLateSuite struct {
	suite.Suite
	ctx           context.Context
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	eventRepoMock *mocks.ProcessedEventRepository
	publisherMock *mocks.SubscriptionEventPublisher
}

func TestProcessSubscriptionLate(t *testing.T) {
	suite.Run(t, new(ProcessSubscriptionLateSuite))
}

func (s *ProcessSubscriptionLateSuite) SetupTest() {
	s.ctx = context.Background()
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
}

func (s *ProcessSubscriptionLateSuite) activeSub(lastEventAt time.Time) entities.Subscription {
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
		lastEventAt.Add(24*time.Hour),
		time.Time{},
		lastEventAt,
	)
}

func (s *ProcessSubscriptionLateSuite) expectRepositories() {
	s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
	s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
}

func (s *ProcessSubscriptionLateSuite) TestExecute() {
	type args struct {
		input input.ProcessSubscriptionLateInput
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(args)
		expect func(error)
	}{
		{
			name: "deve transicionar assinatura para past due com grace end calculado",
			args: args{
				input: func() input.ProcessSubscriptionLateInput {
					now := time.Now().UTC()
					return input.ProcessSubscriptionLateInput{
						EnvelopeID:  "env-001",
						SaleID:      "sale-001",
						OrderID:     "order-001",
						KiwifySubID: "kiwify-sub-001",
						OccurredAt:  now,
					}
				}(),
			},
			setup: func(args args) {
				sub := s.activeSub(args.input.OccurredAt.Add(-2 * time.Hour))
				eventKey := "subscription_late:kiwify-sub-001:" + args.input.OccurredAt.Format("2006-01-02T15:04:05Z07:00")
				expectedGrace := args.input.OccurredAt.Add(3 * 24 * time.Hour)

				s.expectRepositories()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, eventKey, "subscription_late", "kiwify-sub-001", args.input.OccurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ApplyTransition(s.ctx, "sub-001", valueobjects.StatusPastDue, expectedGrace, args.input.OccurredAt).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishPastDue(
						s.ctx,
						mock.Anything,
						mock.MatchedBy(func(updated entities.Subscription) bool {
							return updated.ID() == "sub-001" &&
								updated.Status() == valueobjects.StatusPastDue &&
								updated.GraceEnd().Equal(expectedGrace) &&
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
				input: func() input.ProcessSubscriptionLateInput {
					now := time.Now().UTC()
					return input.ProcessSubscriptionLateInput{
						OrderID:     "order-001",
						KiwifySubID: "kiwify-sub-001",
						OccurredAt:  now,
					}
				}(),
			},
			setup: func(args args) {
				eventKey := "subscription_late:kiwify-sub-001:" + args.input.OccurredAt.Format("2006-01-02T15:04:05Z07:00")

				s.expectRepositories()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, eventKey, "subscription_late", "kiwify-sub-001", args.input.OccurredAt).
					Return(application.ErrEventAlreadyProcessed).
					Once()
			},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, usecases.ErrEventAlreadyProcessed)
			},
		},
		{
			name: "deve retornar erro superseded quando evento estiver stale",
			args: args{
				input: func() input.ProcessSubscriptionLateInput {
					now := time.Now().UTC()
					return input.ProcessSubscriptionLateInput{
						OrderID:     "order-001",
						KiwifySubID: "kiwify-sub-001",
						OccurredAt:  now,
					}
				}(),
			},
			setup: func(args args) {
				sub := s.activeSub(args.input.OccurredAt.Add(time.Hour))
				eventKey := "subscription_late:kiwify-sub-001:" + args.input.OccurredAt.Format("2006-01-02T15:04:05Z07:00")

				s.expectRepositories()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, eventKey, "subscription_late", "kiwify-sub-001", args.input.OccurredAt).
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
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			sut := usecases.NewProcessSubscriptionLate(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
			scenario.setup(scenario.args)

			err := sut.Execute(s.ctx, scenario.args.input)

			scenario.expect(err)
		})
	}
}
