package usecases

import (
	"context"
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

type ProcessSubscriptionLateSuite struct {
	suite.Suite
	ctx           context.Context
	obs           *fake.Provider
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
	s.obs = fake.NewProvider()
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
	return entities.HydrateWithUser(
		"sub-001",
		"user-001",
		funnelToken,
		plan,
		valueobjects.StatusActive,
		lastEventAt.Add(-30*24*time.Hour),
		lastEventAt.Add(24*time.Hour),
		time.Time{},
		lastEventAt,
	)
}

type lateDeps struct {
	factoryMock   *mocks.RepositoryFactory
	publisherMock *mocks.SubscriptionEventPublisher
}

func (s *ProcessSubscriptionLateSuite) TestExecute() {
	type args struct {
		input input.ProcessSubscriptionLateInput
	}

	now := time.Now().UTC()

	scenarios := []struct {
		name         string
		args         args
		dependencies lateDeps
		expect       func(error)
	}{
		{
			name: "deve transicionar assinatura para past due com grace end calculado",
			args: args{
				input: input.ProcessSubscriptionLateInput{
					EnvelopeID:  "env-001",
					SaleID:      "sale-001",
					OrderID:     "order-001",
					KiwifySubID: "kiwify-sub-001",
					OccurredAt:  now,
				},
			},
			dependencies: func() lateDeps {
				sub := s.activeSub(now.Add(-2 * time.Hour))
				eventKey := "subscription_late:kiwify-sub-001:" + now.Format("2006-01-02T15:04:05Z07:00")
				expectedGrace := now.Add(3 * 24 * time.Hour)

				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, eventKey, "subscription_late", "kiwify-sub-001", now).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByKiwifySubID(mock.Anything, "kiwify-sub-001").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ApplyTransition(mock.Anything, "sub-001", valueobjects.StatusPastDue, expectedGrace, now).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishPastDue(
						mock.Anything,
						mock.Anything,
						mock.MatchedBy(func(updated entities.Subscription) bool {
							return updated.ID() == "sub-001" &&
								updated.UserID() == "user-001" &&
								updated.Status() == valueobjects.StatusPastDue &&
								updated.GraceEnd().Equal(expectedGrace) &&
								updated.LastEventAt().Equal(now)
						}),
						"sub-001",
					).
					Return(nil).
					Once()
				return lateDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro de evento ja processado em cenario idempotente",
			args: args{
				input: input.ProcessSubscriptionLateInput{
					OrderID:     "order-001",
					KiwifySubID: "kiwify-sub-001",
					OccurredAt:  now,
				},
			},
			dependencies: func() lateDeps {
				eventKey := "subscription_late:kiwify-sub-001:" + now.Format("2006-01-02T15:04:05Z07:00")

				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, eventKey, "subscription_late", "kiwify-sub-001", now).
					Return(application.ErrEventAlreadyProcessed).
					Once()
				return lateDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, ErrEventAlreadyProcessed)
			},
		},
		{
			name: "deve retornar erro superseded quando evento estiver stale",
			args: args{
				input: input.ProcessSubscriptionLateInput{
					OrderID:     "order-001",
					KiwifySubID: "kiwify-sub-001",
					OccurredAt:  now,
				},
			},
			dependencies: func() lateDeps {
				sub := s.activeSub(now.Add(time.Hour))
				eventKey := "subscription_late:kiwify-sub-001:" + now.Format("2006-01-02T15:04:05Z07:00")

				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, eventKey, "subscription_late", "kiwify-sub-001", now).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByKiwifySubID(mock.Anything, "kiwify-sub-001").
					Return(sub, nil).
					Once()
				s.eventRepoMock.EXPECT().
					MarkSuperseded(mock.Anything, eventKey).
					Return(nil).
					Once()
				return lateDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, ErrEventSuperseded)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewProcessSubscriptionLate(s.uowMock, scenario.dependencies.factoryMock, scenario.dependencies.publisherMock, s.obs)
			err := sut.Execute(s.ctx, scenario.args.input)
			scenario.expect(err)
		})
	}
}
