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

type ProcessSubscriptionRenewedSuite struct {
	suite.Suite
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	eventRepoMock *mocks.ProcessedEventRepository
	planRepoMock  *mocks.PlanRepository
	publisherMock *mocks.SubscriptionEventPublisher
	uc            *usecases.ProcessSubscriptionRenewed
}

func TestProcessSubscriptionRenewed(t *testing.T) {
	suite.Run(t, new(ProcessSubscriptionRenewedSuite))
}

func (s *ProcessSubscriptionRenewedSuite) SetupTest() {
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())
	s.planRepoMock = mocks.NewPlanRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
	s.uc = usecases.NewProcessSubscriptionRenewed(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
}

func (s *ProcessSubscriptionRenewedSuite) activeSub(lastEventAt time.Time) entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	ft, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	periodEnd := lastEventAt.Add(30 * 24 * time.Hour)
	return entities.Hydrate("sub-001", ft, plan, valueobjects.StatusActive,
		lastEventAt.Add(-30*24*time.Hour), periodEnd, time.Time{}, lastEventAt)
}

func (s *ProcessSubscriptionRenewedSuite) TestSucessoExtendePeriodo() {
	now := time.Now().UTC()
	sub := s.activeSub(now.Add(-1 * time.Hour))
	eventKey := "subscription_renewed:kiwify-sub-001:" + now.UTC().Format("2006-01-02T15:04:05Z07:00")

	in := input.ProcessSubscriptionRenewedInput{
		OrderID:         "order-001",
		KiwifySubID:     "kiwify-sub-001",
		KiwifyProductID: "prod-monthly",
		OccurredAt:      now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("PlanRepository", mock.Anything).Return(s.planRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, eventKey, "subscription_renewed", "kiwify-sub-001", now).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil)
	s.subRepoMock.On("ExtendPeriod", mock.Anything, "sub-001", mock.Anything, now).Return(nil)
	expectedPeriodEnd := sub.PeriodEnd().Add(sub.Plan().Duration())
	s.publisherMock.On("PublishRenewed", mock.Anything, mock.Anything,
		mock.MatchedBy(func(renewed entities.Subscription) bool {
			return renewed.Status() == valueobjects.StatusActive &&
				renewed.PeriodEnd().Equal(expectedPeriodEnd) &&
				renewed.LastEventAt().Equal(now)
		}),
		"sub-001",
		sub.PeriodEnd(),
	).Return(nil)

	err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
}

func (s *ProcessSubscriptionRenewedSuite) TestIdempotenciaRetornaErrEventoJaProcessado() {
	now := time.Now().UTC()
	eventKey := "subscription_renewed:kiwify-sub-001:" + now.UTC().Format("2006-01-02T15:04:05Z07:00")

	in := input.ProcessSubscriptionRenewedInput{
		OrderID:     "order-001",
		KiwifySubID: "kiwify-sub-001",
		OccurredAt:  now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("PlanRepository", mock.Anything).Return(s.planRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, eventKey, "subscription_renewed", "kiwify-sub-001", now).Return(interfaces.ErrEventAlreadyProcessed)

	err := s.uc.Execute(context.Background(), in)
	s.Require().Error(err)
	s.True(errors.Is(err, usecases.ErrEventAlreadyProcessed))
}

func (s *ProcessSubscriptionRenewedSuite) pastDueSub(lastEventAt time.Time) entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	ft, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	graceEnd := lastEventAt.Add(3 * 24 * time.Hour)
	return entities.Hydrate("sub-001", ft, plan, valueobjects.StatusPastDue,
		lastEventAt.Add(-30*24*time.Hour), lastEventAt.Add(24*time.Hour), graceEnd, lastEventAt)
}

func (s *ProcessSubscriptionRenewedSuite) TestRenewedStaledQuandoSubEstaPastDueComEventoMaisRecente() {
	recentLastEvent := time.Now().UTC()
	oldOccurredAt := recentLastEvent.Add(-2 * time.Hour)
	sub := s.pastDueSub(recentLastEvent)
	eventKey := "subscription_renewed:kiwify-sub-001:" + oldOccurredAt.UTC().Format("2006-01-02T15:04:05Z07:00")

	in := input.ProcessSubscriptionRenewedInput{
		OrderID:     "order-001",
		KiwifySubID: "kiwify-sub-001",
		OccurredAt:  oldOccurredAt,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("PlanRepository", mock.Anything).Return(s.planRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, eventKey, "subscription_renewed", "kiwify-sub-001", oldOccurredAt).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil)
	s.eventRepoMock.On("MarkSuperseded", mock.Anything, eventKey).Return(nil)

	err := s.uc.Execute(context.Background(), in)
	s.Require().Error(err)
	s.True(errors.Is(err, usecases.ErrEventSuperseded))
}

func (s *ProcessSubscriptionRenewedSuite) TestCriaPlaceholderSeSubNaoExiste() {
	now := time.Now().UTC()
	eventKey := "subscription_renewed:kiwify-sub-001:" + now.UTC().Format("2006-01-02T15:04:05Z07:00")
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	placeholderSub := entities.Hydrate("sub-placeholder", valueobjects.FunnelToken{}, plan, valueobjects.StatusActive,
		now, now.Add(30*24*time.Hour), time.Time{}, now)

	in := input.ProcessSubscriptionRenewedInput{
		OrderID:         "order-001",
		KiwifySubID:     "kiwify-sub-001",
		KiwifyProductID: "prod-monthly",
		OccurredAt:      now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("PlanRepository", mock.Anything).Return(s.planRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, eventKey, "subscription_renewed", "kiwify-sub-001", now).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(entities.Subscription{}, errors.New("not found")).Once()
	s.planRepoMock.On("FindByKiwifyProductID", mock.Anything, "prod-monthly").Return(plan, nil)
	s.subRepoMock.On("UpsertByOrder", mock.Anything, "order-001", mock.Anything, now).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(placeholderSub, nil).Once()
	s.publisherMock.On("PublishRenewed", mock.Anything, mock.Anything, mock.Anything, "sub-placeholder", mock.Anything).Return(nil)

	err = s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
}
