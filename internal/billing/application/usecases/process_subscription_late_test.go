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

type ProcessSubscriptionLateSuite struct {
	suite.Suite
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	eventRepoMock *mocks.ProcessedEventRepository
	publisherMock *mocks.SubscriptionEventPublisher
	uc            *usecases.ProcessSubscriptionLate
}

func TestProcessSubscriptionLate(t *testing.T) {
	suite.Run(t, new(ProcessSubscriptionLateSuite))
}

func (s *ProcessSubscriptionLateSuite) SetupTest() {
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
	s.uc = usecases.NewProcessSubscriptionLate(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
}

func (s *ProcessSubscriptionLateSuite) activeSub(lastEventAt time.Time) entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	ft, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	return entities.Hydrate("sub-001", ft, plan, valueobjects.StatusActive,
		lastEventAt.Add(-30*24*time.Hour), lastEventAt.Add(24*time.Hour), time.Time{}, lastEventAt)
}

func (s *ProcessSubscriptionLateSuite) TestSucessoTransicaoParaPastDue() {
	now := time.Now().UTC()
	sub := s.activeSub(now.Add(-2 * time.Hour))
	eventKey := "subscription_late:kiwify-sub-001:" + now.UTC().Format("2006-01-02T15:04:05Z07:00")

	in := input.ProcessSubscriptionLateInput{
		EnvelopeID:  "env-001",
		SaleID:      "sale-001",
		OrderID:     "order-001",
		KiwifySubID: "kiwify-sub-001",
		OccurredAt:  now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, eventKey, "subscription_late", "kiwify-sub-001", now).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil)
	s.subRepoMock.On("ApplyTransition", mock.Anything, "sub-001", valueobjects.StatusPastDue, mock.Anything, now).Return(nil)
	s.publisherMock.On("PublishPastDue", mock.Anything, mock.Anything, mock.Anything, "sub-001").Return(nil)

	err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
}

func (s *ProcessSubscriptionLateSuite) TestGraceEndEqualLateAtPlusTresDias() {
	now := time.Now().UTC()
	sub := s.activeSub(now.Add(-2 * time.Hour))
	expectedGrace := now.Add(3 * 24 * time.Hour)
	eventKey := "subscription_late:kiwify-sub-001:" + now.UTC().Format("2006-01-02T15:04:05Z07:00")

	in := input.ProcessSubscriptionLateInput{
		OrderID:     "order-001",
		KiwifySubID: "kiwify-sub-001",
		OccurredAt:  now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, eventKey, "subscription_late", "kiwify-sub-001", now).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil)
	s.subRepoMock.On("ApplyTransition", mock.Anything, "sub-001", valueobjects.StatusPastDue, expectedGrace, now).Return(nil)
	s.publisherMock.On("PublishPastDue", mock.Anything, mock.Anything, mock.Anything, "sub-001").Return(nil)

	err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
}

func (s *ProcessSubscriptionLateSuite) TestIdempotenciaRetornaErrEventoJaProcessado() {
	now := time.Now().UTC()
	eventKey := "subscription_late:kiwify-sub-001:" + now.UTC().Format("2006-01-02T15:04:05Z07:00")

	in := input.ProcessSubscriptionLateInput{
		OrderID:     "order-001",
		KiwifySubID: "kiwify-sub-001",
		OccurredAt:  now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, eventKey, "subscription_late", "kiwify-sub-001", now).Return(interfaces.ErrEventAlreadyProcessed)

	err := s.uc.Execute(context.Background(), in)
	s.Require().Error(err)
	s.True(errors.Is(err, usecases.ErrEventAlreadyProcessed))
}

func (s *ProcessSubscriptionLateSuite) TestEventoStaledRetornaSuperseded() {
	now := time.Now().UTC()
	recentLastEvent := now.Add(1 * time.Hour)
	sub := s.activeSub(recentLastEvent)
	eventKey := "subscription_late:kiwify-sub-001:" + now.UTC().Format("2006-01-02T15:04:05Z07:00")

	in := input.ProcessSubscriptionLateInput{
		OrderID:     "order-001",
		KiwifySubID: "kiwify-sub-001",
		OccurredAt:  now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, eventKey, "subscription_late", "kiwify-sub-001", now).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil)
	s.eventRepoMock.On("MarkSuperseded", mock.Anything, eventKey).Return(nil)

	err := s.uc.Execute(context.Background(), in)
	s.Require().Error(err)
	s.True(errors.Is(err, usecases.ErrEventSuperseded))
}
