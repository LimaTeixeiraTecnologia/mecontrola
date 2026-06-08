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

type ReconcileSubscriptionsSuite struct {
	suite.Suite
	ctx              context.Context
	factoryMock      *mocks.RepositoryFactory
	checkpointMock   *mocks.ReconciliationCheckpointRepository
	kiwifyClientMock *mocks.KiwifyClient
	uowMock          *mocks.UnitOfWorkSubscription
	publisherMock    *mocks.SubscriptionEventPublisher
	subRepoMock      *mocks.SubscriptionRepository
	planRepoMock     *mocks.PlanRepository
	eventRepoMock    *mocks.ProcessedEventRepository
}

func TestReconcileSubscriptions(t *testing.T) {
	suite.Run(t, new(ReconcileSubscriptionsSuite))
}

func (s *ReconcileSubscriptionsSuite) SetupTest() {
	s.ctx = context.Background()
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.checkpointMock = mocks.NewReconciliationCheckpointRepository(s.T())
	s.kiwifyClientMock = mocks.NewKiwifyClient(s.T())
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.planRepoMock = mocks.NewPlanRepository(s.T())
	s.eventRepoMock = mocks.NewProcessedEventRepository(s.T())
}

func (s *ReconcileSubscriptionsSuite) newSUT() *usecases.ReconcileSubscriptions {
	saleApproved := usecases.NewProcessSaleApproved(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
	refund := usecases.NewProcessRefundOrChargeback(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
	return usecases.NewReconcileSubscriptions(nil, s.factoryMock, s.kiwifyClientMock, saleApproved, refund, noop.NewProvider())
}

func (s *ReconcileSubscriptionsSuite) monthlyPlan() valueobjects.Plan {
	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	return plan
}

func (s *ReconcileSubscriptionsSuite) activeSubWithID(id, token string) entities.Subscription {
	plan := s.monthlyPlan()
	funnelToken, err := valueobjects.NewFunnelToken(token)
	s.Require().NoError(err)
	now := time.Now().UTC()
	return entities.Hydrate(
		id,
		funnelToken,
		plan,
		valueobjects.StatusActive,
		now,
		now.Add(30*24*time.Hour),
		time.Time{},
		now,
	)
}

func (s *ReconcileSubscriptionsSuite) TestExecute() {
	type args struct {
		input input.ReconcileSubscriptionsInput
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(args)
		expect func(error)
	}{
		{
			name: "deve reconciliar venda aprovada e atualizar checkpoint",
			args: args{
				input: func() input.ReconcileSubscriptionsInput {
					now := time.Now().UTC()
					return input.ReconcileSubscriptionsInput{
						WindowStart: now.Add(-time.Hour),
						WindowEnd:   now,
					}
				}(),
			},
			setup: func(args args) {
				saleTime := args.input.WindowEnd.Add(-30 * time.Minute)
				sale := application.KiwifySale{
					ID:              "sale-001",
					KiwifyProductID: "prod-monthly",
					OrderID:         "order-001",
					FunnelToken:     "token-abc",
					Status:          "paid",
					OccurredAt:      saleTime,
					UpdatedAt:       saleTime,
				}

				s.kiwifyClientMock.EXPECT().
					ListSalesUpdatedSince(s.ctx, args.input.WindowStart, args.input.WindowEnd, 1).
					Return(application.KiwifySalePage{Sales: []application.KiwifySale{sale}, HasMore: false}, nil).
					Once()
				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().PlanRepository(mock.Anything).Return(s.planRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
				s.factoryMock.EXPECT().ReconciliationCheckpointRepository(mock.Anything).Return(s.checkpointMock).Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "compra_aprovada:sale-001", "compra_aprovada", "sale-001", saleTime).
					Return(nil).
					Once()
				s.planRepoMock.EXPECT().
					FindByKiwifyProductID(s.ctx, "prod-monthly").
					Return(s.monthlyPlan(), nil).
					Once()
				s.subRepoMock.EXPECT().
					UpsertByOrder(s.ctx, "order-001", mock.Anything, saleTime).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-001").
					Return(s.activeSubWithID("sub-001", "token-abc"), nil).
					Once()
				s.publisherMock.EXPECT().
					PublishActivated(
						s.ctx,
						mock.Anything,
						mock.Anything,
						"sub-001",
						"token-abc",
						mock.Anything,
						mock.Anything,
						"sale-001",
					).
					Return(nil).
					Once()
				s.checkpointMock.EXPECT().
					Set(s.ctx, "kiwify_sales", args.input.WindowEnd).
					Return(nil).
					Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve nao atualizar checkpoint quando a listagem falhar",
			args: args{
				input: func() input.ReconcileSubscriptionsInput {
					now := time.Now().UTC()
					return input.ReconcileSubscriptionsInput{
						WindowStart: now.Add(-time.Hour),
						WindowEnd:   now,
					}
				}(),
			},
			setup: func(args args) {
				s.kiwifyClientMock.EXPECT().
					ListSalesUpdatedSince(s.ctx, args.input.WindowStart, args.input.WindowEnd, 1).
					Return(application.KiwifySalePage{}, errors.New("kiwify timeout")).
					Once()
			},
			expect: func(err error) {
				s.Error(err)
				s.checkpointMock.AssertNotCalled(s.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "deve nao atualizar checkpoint quando a venda falhar",
			args: args{
				input: func() input.ReconcileSubscriptionsInput {
					now := time.Now().UTC()
					return input.ReconcileSubscriptionsInput{
						WindowStart: now.Add(-time.Hour),
						WindowEnd:   now,
					}
				}(),
			},
			setup: func(args args) {
				sale := application.KiwifySale{
					ID:              "sale-failed",
					KiwifyProductID: "missing-plan",
					OrderID:         "order-failed",
					FunnelToken:     "token-failed",
					Status:          "paid",
					OccurredAt:      args.input.WindowEnd.Add(-time.Minute),
					UpdatedAt:       args.input.WindowEnd.Add(-time.Minute),
				}

				s.kiwifyClientMock.EXPECT().
					ListSalesUpdatedSince(s.ctx, args.input.WindowStart, args.input.WindowEnd, 1).
					Return(application.KiwifySalePage{Sales: []application.KiwifySale{sale}, HasMore: false}, nil).
					Once()
				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().PlanRepository(mock.Anything).Return(s.planRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "compra_aprovada:sale-failed", "compra_aprovada", "sale-failed", sale.OccurredAt).
					Return(nil).
					Once()
				s.planRepoMock.EXPECT().
					FindByKiwifyProductID(s.ctx, "missing-plan").
					Return(valueobjects.Plan{}, errors.New("not found")).
					Once()
			},
			expect: func(err error) {
				s.Error(err)
				s.checkpointMock.AssertNotCalled(s.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "deve encaminhar venda refundada para processamento de refund",
			args: args{
				input: func() input.ReconcileSubscriptionsInput {
					now := time.Now().UTC()
					return input.ReconcileSubscriptionsInput{
						WindowStart: now.Add(-time.Hour),
						WindowEnd:   now,
					}
				}(),
			},
			setup: func(args args) {
				saleTime := args.input.WindowEnd.Add(-30 * time.Minute)
				sale := application.KiwifySale{
					ID:         "sale-002",
					OrderID:    "order-002",
					Status:     "refunded",
					OccurredAt: saleTime,
					UpdatedAt:  saleTime,
				}
				sub := s.activeSubWithID("sub-002", "token-xyz")

				s.kiwifyClientMock.EXPECT().
					ListSalesUpdatedSince(s.ctx, args.input.WindowStart, args.input.WindowEnd, 1).
					Return(application.KiwifySalePage{Sales: []application.KiwifySale{sale}, HasMore: false}, nil).
					Once()
				s.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(s.eventRepoMock).Once()
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
				s.factoryMock.EXPECT().ReconciliationCheckpointRepository(mock.Anything).Return(s.checkpointMock).Once()
				s.eventRepoMock.EXPECT().
					MarkApplied(s.ctx, "refund:sale-002", "compra_reembolsada", "sale-002", saleTime).
					Return(nil).
					Once()
				s.subRepoMock.EXPECT().
					FindByOrderID(s.ctx, "order-002").
					Return(sub, nil).
					Once()
				s.subRepoMock.EXPECT().
					ApplyTransition(s.ctx, "sub-002", valueobjects.StatusRefunded, time.Time{}, saleTime).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishRefunded(s.ctx, mock.Anything, mock.Anything, "sub-002").
					Return(nil).
					Once()
				s.checkpointMock.EXPECT().
					Set(s.ctx, "kiwify_sales", args.input.WindowEnd).
					Return(nil).
					Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			sut := s.newSUT()
			scenario.setup(scenario.args)

			err := sut.Execute(s.ctx, scenario.args.input)

			scenario.expect(err)
		})
	}
}
