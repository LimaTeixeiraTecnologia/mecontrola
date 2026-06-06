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

type ProcessRefundOrChargebackSuite struct {
	suite.Suite
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	eventRepoMock *mocks.ProcessedEventRepository
	publisherMock *mocks.SubscriptionEventPublisher
	uc            *usecases.ProcessRefundOrChargeback
}

func TestProcessRefundOrChargeback(t *testing.T) {
	suite.Run(t, new(ProcessRefundOrChargebackSuite))
}

func (s *ProcessRefundOrChargebackSuite) SetupTest() {
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
	s.uc = usecases.NewProcessRefundOrChargeback(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
}

func (s *ProcessRefundOrChargebackSuite) activeSub() entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	ft, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	now := time.Now().UTC()
	return entities.Hydrate("sub-001", ft, plan, valueobjects.StatusActive,
		now.Add(-30*24*time.Hour), now.Add(24*time.Hour), time.Time{}, now.Add(-1*time.Hour))
}

func (s *ProcessRefundOrChargebackSuite) canceledSub() entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	ft, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	now := time.Now().UTC()
	return entities.Hydrate("sub-001", ft, plan, valueobjects.StatusCanceledPending,
		now.Add(-30*24*time.Hour), now.Add(24*time.Hour), time.Time{}, now.Add(-1*time.Hour))
}

func (s *ProcessRefundOrChargebackSuite) TestSucessoRefundForaAlteraParaRefunded() {
	now := time.Now().UTC()
	sub := s.activeSub()

	in := input.ProcessRefundOrChargebackInput{
		SaleID:     "sale-001",
		OrderID:    "order-001",
		Trigger:    "compra_reembolsada",
		OccurredAt: now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "refund:sale-001", "compra_reembolsada", "sale-001", now).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil)
	s.subRepoMock.On("ApplyTransition", mock.Anything, "sub-001", valueobjects.StatusRefunded, time.Time{}, now).Return(nil)
	s.publisherMock.On("PublishRefunded", mock.Anything, mock.Anything, mock.Anything, "sub-001").Return(nil)

	err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
}

func (s *ProcessRefundOrChargebackSuite) TestRefundEChargebackUsaMesmoEventKey() {
	now := time.Now().UTC()
	sub := s.activeSub()

	inRefund := input.ProcessRefundOrChargebackInput{
		SaleID:     "sale-001",
		OrderID:    "order-001",
		Trigger:    "compra_reembolsada",
		OccurredAt: now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "refund:sale-001", "compra_reembolsada", "sale-001", now).Return(nil).Once()
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil).Once()
	s.subRepoMock.On("ApplyTransition", mock.Anything, "sub-001", valueobjects.StatusRefunded, time.Time{}, now).Return(nil).Once()
	s.publisherMock.On("PublishRefunded", mock.Anything, mock.Anything, mock.Anything, "sub-001").Return(nil).Once()

	err := s.uc.Execute(context.Background(), inRefund)
	s.Require().NoError(err)

	inChargeback := input.ProcessRefundOrChargebackInput{
		SaleID:     "sale-001",
		OrderID:    "order-001",
		Trigger:    "chargeback",
		OccurredAt: now,
	}

	s.eventRepoMock.On("MarkApplied", mock.Anything, "refund:sale-001", "chargeback", "sale-001", now).Return(interfaces.ErrEventAlreadyProcessed).Once()

	err = s.uc.Execute(context.Background(), inChargeback)
	s.Require().Error(err)
	s.True(errors.Is(err, usecases.ErrEventAlreadyProcessed))
}

func (s *ProcessRefundOrChargebackSuite) TestRefundAposCancelForceRefunded() {
	now := time.Now().UTC()
	sub := s.canceledSub()

	in := input.ProcessRefundOrChargebackInput{
		SaleID:     "sale-001",
		OrderID:    "order-001",
		Trigger:    "compra_reembolsada",
		OccurredAt: now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "refund:sale-001", "compra_reembolsada", "sale-001", now).Return(nil)
	s.subRepoMock.On("FindByOrderID", mock.Anything, "order-001").Return(sub, nil)
	s.subRepoMock.On("ApplyTransition", mock.Anything, "sub-001", valueobjects.StatusRefunded, time.Time{}, now).Return(nil)
	s.publisherMock.On("PublishRefunded", mock.Anything, mock.Anything, mock.Anything, "sub-001").Return(nil)

	err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
}

func (s *ProcessRefundOrChargebackSuite) TestIdempotenciaRetornaErrEventoJaProcessado() {
	now := time.Now().UTC()

	in := input.ProcessRefundOrChargebackInput{
		SaleID:     "sale-001",
		OrderID:    "order-001",
		Trigger:    "compra_reembolsada",
		OccurredAt: now,
	}

	s.factoryMock.On("ProcessedEventRepository", mock.Anything).Return(s.eventRepoMock)
	s.factoryMock.On("SubscriptionRepository", mock.Anything).Return(s.subRepoMock)

	s.eventRepoMock.On("MarkApplied", mock.Anything, "refund:sale-001", "compra_reembolsada", "sale-001", now).Return(interfaces.ErrEventAlreadyProcessed)

	err := s.uc.Execute(context.Background(), in)
	s.Require().Error(err)
	s.True(errors.Is(err, usecases.ErrEventAlreadyProcessed))
}
