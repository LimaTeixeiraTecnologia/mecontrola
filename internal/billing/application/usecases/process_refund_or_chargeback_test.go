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

type ProcessRefundOrChargebackSuite struct {
	suite.Suite
	ctx           context.Context
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

func (s *ProcessRefundOrChargebackSuite) expectRepositories(times int) {
	s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Times(times)
	s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Times(times)
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

	scenarios := []struct {
		name   string
		args   args
		setup  func(args)
		expect func(result)
	}{
		{
			name: "deve transicionar assinatura ativa para refunded",
			args: args{
				inputs: []input.ProcessRefundOrChargebackInput{
					{
						SaleID:     "sale-001",
						OrderID:    "order-001",
						Trigger:    "order_refunded",
						OccurredAt: time.Now().UTC(),
					},
				},
			},
			setup: func(args args) {
				sub := s.activeSub()
				occurredAt := args.inputs[0].OccurredAt

				s.expectRepositories(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "order_refunded:sale-001", "order_refunded", "sale-001", occurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ApplyTransition(s.ctx, "sub-001", valueobjects.StatusRefunded, time.Time{}, occurredAt).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishRefunded(
						s.ctx,
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
			},
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
						OccurredAt: time.Now().UTC(),
					},
					{
						SaleID:     "sale-001",
						OrderID:    "order-001",
						Trigger:    "chargeback",
						OccurredAt: time.Now().UTC().Add(time.Hour),
					},
				},
			},
			setup: func(args args) {
				sub := s.activeSub()
				s.expectRepositories(len(args.inputs))
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "order_refunded:sale-001", "order_refunded", "sale-001", args.inputs[0].OccurredAt).
					Return(nil).
					Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "chargeback:sale-001", "chargeback", "sale-001", args.inputs[1].OccurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(sub, nil).
					Twice()
				s.subRepoMock.EXPECT().
					ApplyTransition(s.ctx, "sub-001", valueobjects.StatusRefunded, time.Time{}, mock.Anything).
					Return(nil).
					Twice()
				s.publisherMock.EXPECT().
					PublishRefunded(s.ctx, mock.Anything, mock.Anything, "sub-001").
					Return(nil).
					Twice()
			},
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
						OccurredAt: time.Now().UTC(),
					},
					{
						SaleID:     "sale-001",
						OrderID:    "order-001",
						Trigger:    "chargeback",
						OccurredAt: time.Now().UTC(),
					},
				},
			},
			setup: func(args args) {
				sub := s.activeSub()
				s.expectRepositories(len(args.inputs))
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "order_refunded:sale-001", "order_refunded", "sale-001", args.inputs[0].OccurredAt).
					Return(nil).
					Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "chargeback:sale-001", "chargeback", "sale-001", args.inputs[1].OccurredAt).
					Return(application.ErrEventAlreadyProcessed).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ApplyTransition(s.ctx, "sub-001", valueobjects.StatusRefunded, time.Time{}, args.inputs[0].OccurredAt).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishRefunded(s.ctx, mock.Anything, mock.Anything, "sub-001").
					Return(nil).
					Once()
			},
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
						OccurredAt: time.Now().UTC(),
					},
				},
			},
			setup: func(args args) {
				sub := s.canceledSub()
				occurredAt := args.inputs[0].OccurredAt

				s.expectRepositories(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "order_refunded:sale-001", "order_refunded", "sale-001", occurredAt).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ApplyTransition(s.ctx, "sub-001", valueobjects.StatusRefunded, time.Time{}, occurredAt).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishRefunded(s.ctx, mock.Anything, mock.Anything, "sub-001").
					Return(nil).
					Once()
			},
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
						OccurredAt: time.Now().UTC(),
					},
				},
			},
			setup: func(args args) {
				s.expectRepositories(1)
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "order_refunded:sale-001", "order_refunded", "sale-001", args.inputs[0].OccurredAt).
					Return(application.ErrEventAlreadyProcessed).
					Once()
			},
			expect: func(result result) {
				s.Error(result.err)
				s.ErrorIs(result.err, usecases.ErrEventAlreadyProcessed)
				s.Zero(result.transitions)
				s.Zero(result.duplicates)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			sut := usecases.NewProcessRefundOrChargeback(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
			scenario.setup(scenario.args)

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
				if errors.Is(err, usecases.ErrEventAlreadyProcessed) {
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
