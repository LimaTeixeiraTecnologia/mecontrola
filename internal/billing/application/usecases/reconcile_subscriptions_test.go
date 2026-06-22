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

type ReconcileSubscriptionsSuite struct {
	suite.Suite
	ctx              context.Context
	obs              *fake.Provider
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
	s.obs = fake.NewProvider()
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

type reconcileDeps struct {
	checkpointMock   *mocks.ReconciliationCheckpointRepository
	kiwifyClientMock *mocks.KiwifyClient
	factoryMock      *mocks.RepositoryFactory
	publisherMock    *mocks.SubscriptionEventPublisher
	subRepoMock      *mocks.SubscriptionRepository
	planRepoMock     *mocks.PlanRepository
	eventRepoMock    *mocks.ProcessedEventRepository
}

func (s *ReconcileSubscriptionsSuite) newDeps() reconcileDeps {
	return reconcileDeps{
		checkpointMock:   mocks.NewReconciliationCheckpointRepository(s.T()),
		kiwifyClientMock: mocks.NewKiwifyClient(s.T()),
		factoryMock:      mocks.NewRepositoryFactory(s.T()),
		publisherMock:    mocks.NewSubscriptionEventPublisher(s.T()),
		subRepoMock:      mocks.NewSubscriptionRepository(s.T()),
		planRepoMock:     mocks.NewPlanRepository(s.T()),
		eventRepoMock:    mocks.NewProcessedEventRepository(s.T()),
	}
}

func (s *ReconcileSubscriptionsSuite) buildSUT(d reconcileDeps) *ReconcileSubscriptions {
	saleApproved := NewProcessSaleApproved(s.uowMock, d.factoryMock, d.publisherMock, s.obs)
	refund := NewProcessRefundOrChargeback(s.uowMock, d.factoryMock, d.publisherMock, s.obs)
	return NewReconcileSubscriptions(d.checkpointMock, d.kiwifyClientMock, saleApproved, refund, s.obs)
}

func (s *ReconcileSubscriptionsSuite) TestExecute() {
	type args struct {
		input input.ReconcileSubscriptionsInput
	}

	now := time.Now().UTC()

	scenarios := []struct {
		name         string
		args         args
		dependencies reconcileDeps
		expect       func(d reconcileDeps, err error)
	}{
		{
			name: "deve reconciliar venda aprovada e atualizar checkpoint",
			args: args{
				input: input.ReconcileSubscriptionsInput{
					WindowStart: now.Add(-time.Hour),
					WindowEnd:   now,
				},
			},
			dependencies: func() reconcileDeps {
				d := s.newDeps()
				saleTime := now.Add(-30 * time.Minute)
				sale := application.KiwifySale{
					ID:              "sale-001",
					KiwifyProductID: "prod-monthly",
					OrderID:         "order-001",
					SubscriptionID:  "kiwify-sub-001",
					FunnelToken:     "token-abc",
					Status:          "paid",
					OccurredAt:      saleTime,
					UpdatedAt:       saleTime,
				}

				d.kiwifyClientMock.EXPECT().
					ListSalesUpdatedSince(mock.Anything, now.Add(-time.Hour), now, 1).
					Return(application.KiwifySalePage{Sales: []application.KiwifySale{sale}, HasMore: false}, nil).
					Once()
				d.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(d.eventRepoMock).Once()
				d.factoryMock.EXPECT().PlanRepository(mock.Anything).Return(d.planRepoMock).Once()
				d.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(d.subRepoMock).Once()
				d.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_approved:sale-001", "order_approved", "sale-001", saleTime).
					Return(nil).
					Once()
				d.planRepoMock.EXPECT().
					FindByKiwifyProductID(mock.Anything, "prod-monthly").
					Return(s.monthlyPlan(), nil).
					Once()
				d.subRepoMock.EXPECT().
					UpsertByOrder(mock.Anything, mock.Anything).
					Return(nil).
					Once()
				d.subRepoMock.EXPECT().
					FindByOrderID(mock.Anything, "order-001").
					Return(s.activeSubWithID("sub-001", "token-abc"), nil).
					Once()
				d.publisherMock.EXPECT().
					PublishActivated(
						mock.Anything,
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
				d.checkpointMock.EXPECT().
					Set(mock.Anything, "kiwify_sales", now).
					Return(nil).
					Once()
				return d
			}(),
			expect: func(d reconcileDeps, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve nao atualizar checkpoint quando a listagem falhar",
			args: args{
				input: input.ReconcileSubscriptionsInput{
					WindowStart: now.Add(-time.Hour),
					WindowEnd:   now,
				},
			},
			dependencies: func() reconcileDeps {
				d := s.newDeps()
				d.kiwifyClientMock.EXPECT().
					ListSalesUpdatedSince(mock.Anything, now.Add(-time.Hour), now, 1).
					Return(application.KiwifySalePage{}, errors.New("kiwify timeout")).
					Once()
				return d
			}(),
			expect: func(d reconcileDeps, err error) {
				s.Error(err)
				d.checkpointMock.AssertNotCalled(s.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "deve nao atualizar checkpoint quando a venda falhar",
			args: args{
				input: input.ReconcileSubscriptionsInput{
					WindowStart: now.Add(-time.Hour),
					WindowEnd:   now,
				},
			},
			dependencies: func() reconcileDeps {
				d := s.newDeps()
				saleTime := now.Add(-time.Minute)
				sale := application.KiwifySale{
					ID:              "sale-failed",
					KiwifyProductID: "missing-plan",
					OrderID:         "order-failed",
					SubscriptionID:  "kiwify-sub-failed",
					FunnelToken:     "token-failed",
					Status:          "paid",
					OccurredAt:      saleTime,
					UpdatedAt:       saleTime,
				}

				d.kiwifyClientMock.EXPECT().
					ListSalesUpdatedSince(mock.Anything, now.Add(-time.Hour), now, 1).
					Return(application.KiwifySalePage{Sales: []application.KiwifySale{sale}, HasMore: false}, nil).
					Once()
				d.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(d.eventRepoMock).Once()
				d.factoryMock.EXPECT().PlanRepository(mock.Anything).Return(d.planRepoMock).Once()
				d.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(d.subRepoMock).Once()
				d.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_approved:sale-failed", "order_approved", "sale-failed", saleTime).
					Return(nil).
					Once()
				d.planRepoMock.EXPECT().
					FindByKiwifyProductID(mock.Anything, "missing-plan").
					Return(valueobjects.Plan{}, errors.New("not found")).
					Once()
				return d
			}(),
			expect: func(d reconcileDeps, err error) {
				s.Error(err)
				d.checkpointMock.AssertNotCalled(s.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "deve encaminhar venda refundada para processamento de refund",
			args: args{
				input: input.ReconcileSubscriptionsInput{
					WindowStart: now.Add(-time.Hour),
					WindowEnd:   now,
				},
			},
			dependencies: func() reconcileDeps {
				d := s.newDeps()
				saleTime := now.Add(-30 * time.Minute)
				sale := application.KiwifySale{
					ID:         "sale-002",
					OrderID:    "order-002",
					Status:     "refunded",
					OccurredAt: saleTime,
					UpdatedAt:  saleTime,
				}
				sub := s.activeSubWithID("sub-002", "token-xyz")

				d.kiwifyClientMock.EXPECT().
					ListSalesUpdatedSince(mock.Anything, now.Add(-time.Hour), now, 1).
					Return(application.KiwifySalePage{Sales: []application.KiwifySale{sale}, HasMore: false}, nil).
					Once()
				d.factoryMock.EXPECT().ProcessedEventRepository(mock.Anything).Return(d.eventRepoMock).Once()
				d.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(d.subRepoMock).Once()
				d.eventRepoMock.EXPECT().
					MarkApplied(mock.Anything, "order_refunded:sale-002", "order_refunded", "sale-002", saleTime).
					Return(nil).
					Once()
				d.subRepoMock.EXPECT().
					FindByOrderID(mock.Anything, "order-002").
					Return(sub, nil).
					Once()
				d.subRepoMock.EXPECT().
					ApplyTransition(mock.Anything, "sub-002", valueobjects.StatusRefunded, time.Time{}, saleTime).
					Return(nil).
					Once()
				d.publisherMock.EXPECT().
					PublishRefunded(mock.Anything, mock.Anything, mock.Anything, "sub-002").
					Return(nil).
					Once()
				d.checkpointMock.EXPECT().
					Set(mock.Anything, "kiwify_sales", now).
					Return(nil).
					Once()
				return d
			}(),
			expect: func(d reconcileDeps, err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := s.buildSUT(scenario.dependencies)
			err := sut.Execute(s.ctx, scenario.args.input)
			scenario.expect(scenario.dependencies, err)
		})
	}
}

func (s *ReconcileSubscriptionsSuite) TestMaxPagesGuard() {
	in := input.ReconcileSubscriptionsInput{
		WindowStart: time.Now().UTC().Add(-time.Hour),
		WindowEnd:   time.Now().UTC(),
	}

	s.kiwifyClientMock.EXPECT().
		ListSalesUpdatedSince(mock.Anything, in.WindowStart, in.WindowEnd, mock.AnythingOfType("int")).
		Return(application.KiwifySalePage{Sales: nil, HasMore: true}, nil).
		Times(1000)

	d := reconcileDeps{
		checkpointMock:   s.checkpointMock,
		kiwifyClientMock: s.kiwifyClientMock,
		factoryMock:      s.factoryMock,
		publisherMock:    s.publisherMock,
		subRepoMock:      s.subRepoMock,
		planRepoMock:     s.planRepoMock,
		eventRepoMock:    s.eventRepoMock,
	}
	sut := s.buildSUT(d)
	err := sut.Execute(s.ctx, in)
	s.Require().Error(err)
	s.ErrorIs(err, ErrReconcileMaxPagesExceeded)
}
