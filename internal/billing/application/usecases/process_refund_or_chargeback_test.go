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

type ProcessRefundOrChargebackSuite struct {
	suite.Suite
	ctx           context.Context
	obs           *fake.Provider
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	eventRepoMock *mocks.ProcessedEventRepository
	publisherMock *mocks.SubscriptionEventPublisher
}

func TestProcessRefundOrChargeback(t *testing.T) {
	suite.Run(t, new(ProcessRefundOrChargebackSuite))
}

func (s *ProcessRefundOrChargebackSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
}

func (s *ProcessRefundOrChargebackSuite) activeSub() entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	funnelToken, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	now := time.Now().UTC()
	return entities.HydrateWithUser(
		"sub-001",
		"user-001",
		funnelToken,
		plan,
		valueobjects.StatusActive,
		now.Add(-30*24*time.Hour),
		now.Add(24*time.Hour),
		time.Time{},
		now.Add(-time.Hour),
	)
}

func (s *ProcessRefundOrChargebackSuite) canceledSub() entities.Subscription {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	funnelToken, err := valueobjects.NewFunnelToken("token-abc")
	s.Require().NoError(err)
	now := time.Now().UTC()
	return entities.HydrateWithUser(
		"sub-001",
		"user-001",
		funnelToken,
		plan,
		valueobjects.StatusCanceledPending,
		now.Add(-30*24*time.Hour),
		now.Add(24*time.Hour),
		time.Time{},
		now.Add(-time.Hour),
	)
}

type refundDeps struct {
	factoryMock   *mocks.RepositoryFactory
	publisherMock *mocks.SubscriptionEventPublisher
}

func (s *ProcessRefundOrChargebackSuite) TestExecute() {
	type args struct {
		inputs []input.ProcessRefundOrChargebackInput
	}

	type result struct {
		err         error
		transitions int
		duplicates  int
	}

	now := time.Now().UTC()

	scenarios := []struct {
		name         string
		args         args
		dependencies refundDeps
		expect       func(result)
	}{
		{
			name: "deve transicionar assinatura ativa para refunded",
			args: args{
				inputs: []input.ProcessRefundOrChargebackInput{
					{
						SaleID:     "sale-001",
						OrderID:    "order-001",
						Trigger:    "order_refunded",
						OccurredAt: now,
					},
				},
			},
			dependencies: func() refundDeps {
				sub := s.activeSub()
				occurredAt := now

				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(1)
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_refunded:sale-001", "order_refunded", "sale-001", occurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(mock.Anything, "order-001").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ApplyTransition(mock.Anything, "sub-001", valueobjects.StatusRefunded, time.Time{}, occurredAt).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishRefunded(
						mock.Anything,
						mock.Anything,
						mock.MatchedBy(func(updated entities.Subscription) bool {
							return updated.ID() == "sub-001" &&
								updated.UserID() == "user-001" &&
								updated.Status() == valueobjects.StatusRefunded &&
								updated.GraceEnd().IsZero() &&
								updated.LastEventAt().Equal(occurredAt)
						}),
						"sub-001",
					).
					Return(nil).
					Once()
				return refundDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(result result) {
				s.NoError(result.err)
				s.Equal(1, result.transitions)
				s.Zero(result.duplicates)
			},
		},
		{
			name: "deve permitir reaplicar chargeback apos order_refunded pois chaves de evento sao independentes (regressao #4)",
			args: args{
				inputs: []input.ProcessRefundOrChargebackInput{
					{
						SaleID:     "sale-001",
						OrderID:    "order-001",
						Trigger:    "order_refunded",
						OccurredAt: now,
					},
					{
						SaleID:     "sale-001",
						OrderID:    "order-001",
						Trigger:    "chargeback",
						OccurredAt: now.Add(time.Hour),
					},
				},
			},
			dependencies: func() refundDeps {
				sub := s.activeSub()
				chargebackAt := now.Add(time.Hour)

				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(2)
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(2)
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_refunded:sale-001", "order_refunded", "sale-001", now).
					Return(nil).
					Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "chargeback:sale-001", "chargeback", "sale-001", chargebackAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(mock.Anything, "order-001").
					Return(sub, nil).
					Twice()
				s.subRepoMock.EXPECT().
					ApplyTransition(mock.Anything, "sub-001", valueobjects.StatusRefunded, time.Time{}, mock.Anything).
					Return(nil).
					Twice()
				s.publisherMock.EXPECT().
					PublishRefunded(mock.Anything, mock.Anything, mock.Anything, "sub-001").
					Return(nil).
					Twice()
				return refundDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(result result) {
				s.NoError(result.err)
				s.Equal(2, result.transitions)
				s.Zero(result.duplicates)
			},
		},
		{
			name: "deve usar a mesma chave de evento para refund e chargeback",
			args: args{
				inputs: []input.ProcessRefundOrChargebackInput{
					{
						SaleID:     "sale-001",
						OrderID:    "order-001",
						Trigger:    "order_refunded",
						OccurredAt: now,
					},
					{
						SaleID:     "sale-001",
						OrderID:    "order-001",
						Trigger:    "chargeback",
						OccurredAt: now,
					},
				},
			},
			dependencies: func() refundDeps {
				sub := s.activeSub()

				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(2)
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(2)
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_refunded:sale-001", "order_refunded", "sale-001", now).
					Return(nil).
					Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "chargeback:sale-001", "chargeback", "sale-001", now).
					Return(application.ErrEventAlreadyProcessed).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(mock.Anything, "order-001").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ApplyTransition(mock.Anything, "sub-001", valueobjects.StatusRefunded, time.Time{}, now).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishRefunded(mock.Anything, mock.Anything, mock.Anything, "sub-001").
					Return(nil).
					Once()
				return refundDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(result result) {
				s.NoError(result.err)
				s.Equal(1, result.transitions)
				s.Equal(1, result.duplicates)
			},
		},
		{
			name: "deve forcar refunded quando reembolso ocorrer apos cancelamento",
			args: args{
				inputs: []input.ProcessRefundOrChargebackInput{
					{
						SaleID:     "sale-001",
						OrderID:    "order-001",
						Trigger:    "order_refunded",
						OccurredAt: now,
					},
				},
			},
			dependencies: func() refundDeps {
				sub := s.canceledSub()

				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(1)
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_refunded:sale-001", "order_refunded", "sale-001", now).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(mock.Anything, "order-001").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ApplyTransition(mock.Anything, "sub-001", valueobjects.StatusRefunded, time.Time{}, now).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishRefunded(mock.Anything, mock.Anything, mock.Anything, "sub-001").
					Return(nil).
					Once()
				return refundDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(result result) {
				s.NoError(result.err)
				s.Equal(1, result.transitions)
				s.Zero(result.duplicates)
			},
		},
		{
			name: "deve retornar erro de evento ja processado em cenario idempotente",
			args: args{
				inputs: []input.ProcessRefundOrChargebackInput{
					{
						SaleID:     "sale-001",
						OrderID:    "order-001",
						Trigger:    "order_refunded",
						OccurredAt: now,
					},
				},
			},
			dependencies: func() refundDeps {
				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(1)
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_refunded:sale-001", "order_refunded", "sale-001", now).
					Return(application.ErrEventAlreadyProcessed).
					Once()
				return refundDeps{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(result result) {
				s.Error(result.err)
				s.ErrorIs(result.err, ErrEventAlreadyProcessed)
				s.Zero(result.transitions)
				s.Zero(result.duplicates)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewProcessRefundOrChargeback(s.uowMock, scenario.dependencies.factoryMock, scenario.dependencies.publisherMock, s.obs)

			result := result{}
			for _, currentInput := range scenario.args.inputs {
				err := sut.Execute(s.ctx, currentInput)
				if len(scenario.args.inputs) == 1 {
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
				if errors.Is(err, ErrEventAlreadyProcessed) {
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
