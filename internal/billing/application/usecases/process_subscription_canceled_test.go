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

type ProcessSubscriptionCanceledSuite struct {
	suite.Suite
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	eventRepoMock *mocks.ProcessedEventRepository
	publisherMock *mocks.SubscriptionEventPublisher
	uc            *usecases.ProcessSubscriptionCanceled
}

func TestProcessSubscriptionCanceled(t *testing.T) {
	suite.Run(t, new(ProcessSubscriptionCanceledSuite))
}

func (s *ProcessSubscriptionCanceledSuite) SetupTest() {
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
	s.uc = usecases.NewProcessSubscriptionCanceled(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
}

func (s *ProcessSubscriptionCanceledSuite) activeSub(lastEventAt time.Time) entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	ft, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	periodEnd := lastEventAt.Add(30 * 24 * time.Hour)
	return entities.Hydrate("sub-001", ft, plan, valueobjects.StatusActive,
		lastEventAt.Add(-30*24*time.Hour), periodEnd, time.Time{}, lastEventAt)
}

func (s *ProcessSubscriptionCanceledSuite) TestSucessoTransicaoParaCanceledPending() {
	now := time.Now().UTC()
	sub := s.activeSub(now.Add(-2 * time.Hour))

	in := input.ProcessSubscriptionCanceledInput{
		OrderID:     "order-001",
		KiwifySubID: "kiwify-sub-001",
		OccurredAt:  now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "subscription_canceled:kiwify-sub-001", "subscription_canceled", "kiwify-sub-001", now).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil)
	s.subRepoMock.On("ApplyTransition", mock.Anything, "sub-001", valueobjects.StatusCanceledPending, time.Time{}, now).Return(nil)
	s.publisherMock.On("PublishCanceled", mock.Anything, mock.Anything, mock.Anything, "sub-001").Return(nil)

	err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
}

func (s *ProcessSubscriptionCanceledSuite) TestPeriodEndPreservadoNoCancelamento() {
	now := time.Now().UTC()
	sub := s.activeSub(now.Add(-1 * time.Hour))

	in := input.ProcessSubscriptionCanceledInput{
		OrderID:     "order-001",
		KiwifySubID: "kiwify-sub-001",
		OccurredAt:  now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "subscription_canceled:kiwify-sub-001", "subscription_canceled", "kiwify-sub-001", now).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil)

	var capturedGrace time.Time
	s.subRepoMock.On("ApplyTransition", mock.Anything, "sub-001", valueobjects.StatusCanceledPending, mock.MatchedBy(func(t time.Time) bool {
		capturedGrace = t
		return true
	}), now).Return(nil)
	s.publisherMock.On("PublishCanceled", mock.Anything, mock.Anything, mock.Anything, "sub-001").Return(nil)

	err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
	s.True(capturedGrace.IsZero(), "grace_end deve ser zero no cancelamento")
}

func (s *ProcessSubscriptionCanceledSuite) TestIdempotenciaRetornaErrEventoJaProcessado() {
	now := time.Now().UTC()

	in := input.ProcessSubscriptionCanceledInput{
		OrderID:     "order-001",
		KiwifySubID: "kiwify-sub-001",
		OccurredAt:  now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "subscription_canceled:kiwify-sub-001", "subscription_canceled", "kiwify-sub-001", now).Return(interfaces.ErrEventAlreadyProcessed)

	err := s.uc.Execute(context.Background(), in)
	s.Require().Error(err)
	s.True(errors.Is(err, usecases.ErrEventAlreadyProcessed))
}
