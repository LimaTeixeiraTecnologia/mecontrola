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

type ProcessSubscriptionRenewedSuite struct {
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

func TestProcessSubscriptionRenewed(t *testing.T) {
	suite.Run(t, new(ProcessSubscriptionRenewedSuite))
}

func (s *ProcessSubscriptionRenewedSuite) SetupTest() {
	s.obs = fake.NewProvider()
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
	return entities.HydrateWithUser(
		"sub-001",
		"user-001",
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
	return entities.HydrateWithUser(
		"sub-001",
		"user-001",
		funnelToken,
		plan,
		valueobjects.StatusPastDue,
		lastEventAt.Add(-30*24*time.Hour),
		lastEventAt.Add(24*time.Hour),
		lastEventAt.Add(3*24*time.Hour),
		lastEventAt,
	)
}

type renewedDeps struct {
	factoryMock   *mocks.RepositoryFactory
	publisherMock *mocks.SubscriptionEventPublisher
}

func (s *ProcessSubscriptionRenewedSuite) TestExecute() {
	type args struct {
		input input.ProcessSubscriptionRenewedInput
	}

	now := time.Now().UTC()

	scenarios := []struct {
		name         string
		args         args
		dependencies renewedDeps
		expect       func(error)
	}{
		{
			name: "deve retornar erro quando kiwify subscription id for invalido",
			args: args{
				input: input.ProcessSubscriptionRenewedInput{
					OrderID:     "order-001",
					KiwifySubID: "   ",
					OccurredAt:  now,
				},
			},
			dependencies: renewedDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, ErrKiwifySubscriptionIDInvalid)
			},
		},
		{
			name: "deve estender periodo de assinatura ativa",
			args: args{
				input: input.ProcessSubscriptionRenewedInput{
					OrderID:         "order-001",
					KiwifySubID:     "kiwify-sub-001",
					KiwifyProductID: "prod-monthly",
					OccurredAt:      now,
				},
			},
			dependencies: func() renewedDeps {
				sub := s.activeSub(now.Add(-time.Hour))
				eventKey := "subscription_renewed:kiwify-sub-001:" + now.Format("2006-01-02T15:04:05Z07:00")
				expectedPeriodEnd := sub.PeriodEnd().Add(sub.Plan().Duration())

				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, eventKey, "subscription_renewed", "kiwify-sub-001", now).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByKiwifySubID(mock.Anything, "kiwify-sub-001").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ExtendPeriod(mock.Anything, "sub-001", expectedPeriodEnd, now).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishRenewed(
						mock.Anything,
						mock.Anything,
						mock.MatchedBy(func(renewed entities.Subscription) bool {
							return renewed.ID() == "sub-001" &&
								renewed.UserID() == "user-001" &&
								renewed.Status() == valueobjects.StatusActive &&
								renewed.PeriodEnd().Equal(expectedPeriodEnd) &&
								renewed.LastEventAt().Equal(now)
						}),
						"sub-001",
						sub.PeriodEnd(),
					).
					Return(nil).
					Once()
				return renewedDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro de evento ja processado em cenario idempotente",
			args: args{
				input: input.ProcessSubscriptionRenewedInput{
					OrderID:     "order-001",
					KiwifySubID: "kiwify-sub-001",
					OccurredAt:  now,
				},
			},
			dependencies: func() renewedDeps {
				eventKey := "subscription_renewed:kiwify-sub-001:" + now.Format("2006-01-02T15:04:05Z07:00")

				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, eventKey, "subscription_renewed", "kiwify-sub-001", now).
					Return(application.ErrEventAlreadyProcessed).
					Once()
				return renewedDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, ErrEventAlreadyProcessed)
			},
		},
		{
			name: "deve marcar evento stale como superseded quando houver evento mais recente",
			args: args{
				input: input.ProcessSubscriptionRenewedInput{
					OrderID:     "order-001",
					KiwifySubID: "kiwify-sub-001",
					OccurredAt:  now,
				},
			},
			dependencies: func() renewedDeps {
				sub := s.pastDueSub(now.Add(2 * time.Hour))
				eventKey := "subscription_renewed:kiwify-sub-001:" + now.Format("2006-01-02T15:04:05Z07:00")

				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, eventKey, "subscription_renewed", "kiwify-sub-001", now).
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
				return renewedDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, ErrEventSuperseded)
			},
		},
		{
			name: "deve retornar erro quando renovacao chegar sem assinatura base por kiwify_sub_id",
			args: args{
				input: input.ProcessSubscriptionRenewedInput{
					OrderID:         "order-001",
					KiwifySubID:     "kiwify-sub-001",
					KiwifyProductID: "prod-monthly",
					OccurredAt:      now,
				},
			},
			dependencies: func() renewedDeps {
				eventKey := "subscription_renewed:kiwify-sub-001:" + now.Format("2006-01-02T15:04:05Z07:00")

				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, eventKey, "subscription_renewed", "kiwify-sub-001", now).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByKiwifySubID(mock.Anything, "kiwify-sub-001").
					Return(entities.Subscription{}, errors.New("not found")).
					Once()
				return renewedDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(err error) {
				s.ErrorIs(err, ErrRenewedWithoutBaseSubscription)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewProcessSubscriptionRenewed(s.uowMock, scenario.dependencies.factoryMock, scenario.dependencies.publisherMock, s.obs)
			err := sut.Execute(s.ctx, scenario.args.input)
			scenario.expect(err)
		})
	}
}
