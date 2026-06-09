//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"
	billingpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type SubscriptionRepositorySuite struct {
	suite.Suite
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func TestSubscriptionRepositorySuite(t *testing.T) {
	suite.Run(t, new(SubscriptionRepositorySuite))
}

func (s *SubscriptionRepositorySuite) SetupTest() {}

func (s *SubscriptionRepositorySuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *SubscriptionRepositorySuite) newRepo() interfaces.SubscriptionRepository {
	return s.factory.SubscriptionRepository(s.mgr.DBTX(context.Background()))
}

func (s *SubscriptionRepositorySuite) newPlan(code valueobjects.PlanCode, days int) valueobjects.Plan {
	plan, err := valueobjects.NewPlan(string(code), days)
	s.Require().NoError(err)
	return plan
}

func (s *SubscriptionRepositorySuite) newFunnelToken(raw string) valueobjects.FunnelToken {
	ft, err := valueobjects.NewFunnelToken(raw)
	s.Require().NoError(err)
	return ft
}

func (s *SubscriptionRepositorySuite) TestRepositoryOperations() {
	scenarios := []struct {
		name   string
		expect func(context.Context, interfaces.SubscriptionRepository)
	}{
		{
			name: "deve inserir subscription no upsert por order",
			expect: func(ctx context.Context, repo interfaces.SubscriptionRepository) {
				plan := s.newPlan(valueobjects.PlanCodeMonthly, 30)
				ft := s.newFunnelToken("token-upsert-001")
				sub := entities.NewSubscription(plan, ft)
				occurredAt := time.Now().UTC().Truncate(time.Millisecond)
				s.Require().NoError(sub.Activate(occurredAt))
				orderID := "order-upsert-001"
				s.Require().NoError(repo.UpsertByOrder(ctx, interfaces.UpsertByOrderParams{OrderID: orderID, Subscription: sub, PeriodStart: occurredAt}))
				found, err := repo.FindByOrderID(ctx, orderID)
				s.Require().NoError(err)
				s.Assert().Equal(valueobjects.StatusActive, found.Status())
				s.Assert().Equal(ft.String(), found.FunnelToken().String())
			},
		},
		{
			name: "deve retornar erro quando a subscription nao existir",
			expect: func(ctx context.Context, repo interfaces.SubscriptionRepository) {
				_, err := repo.FindByOrderID(ctx, "nonexistent-order-999")
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, billingpostgres.ErrSubscriptionNotFound))
			},
		},
		{
			name: "deve atualizar o period end ao estender a assinatura",
			expect: func(ctx context.Context, repo interfaces.SubscriptionRepository) {
				plan := s.newPlan(valueobjects.PlanCodeMonthly, 30)
				ft := s.newFunnelToken("token-extend-001")
				sub := entities.NewSubscription(plan, ft)
				occurredAt := time.Now().UTC().Truncate(time.Millisecond)
				s.Require().NoError(sub.Activate(occurredAt))
				orderID := "order-extend-001"
				s.Require().NoError(repo.UpsertByOrder(ctx, interfaces.UpsertByOrderParams{OrderID: orderID, Subscription: sub, PeriodStart: occurredAt}))
				found, err := repo.FindByOrderID(ctx, orderID)
				s.Require().NoError(err)
				newEnd := time.Now().UTC().Add(60 * 24 * time.Hour).Truncate(time.Millisecond)
				renewedAt := time.Now().UTC().Truncate(time.Millisecond)
				s.Require().NoError(repo.ExtendPeriod(ctx, found.ID(), newEnd, renewedAt))
				updated, err := repo.FindByOrderID(ctx, orderID)
				s.Require().NoError(err)
				s.Assert().Equal(valueobjects.StatusActive, updated.Status())
				s.Assert().WithinDuration(newEnd, updated.PeriodEnd(), time.Second)
			},
		},
		{
			name: "deve aplicar transicao para past due",
			expect: func(ctx context.Context, repo interfaces.SubscriptionRepository) {
				plan := s.newPlan(valueobjects.PlanCodeMonthly, 30)
				ft := s.newFunnelToken("token-pastdue-001")
				sub := entities.NewSubscription(plan, ft)
				occurredAt := time.Now().UTC().Truncate(time.Millisecond)
				s.Require().NoError(sub.Activate(occurredAt))
				orderID := "order-pastdue-001"
				s.Require().NoError(repo.UpsertByOrder(ctx, interfaces.UpsertByOrderParams{OrderID: orderID, Subscription: sub, PeriodStart: occurredAt}))
				found, err := repo.FindByOrderID(ctx, orderID)
				s.Require().NoError(err)
				lateAt := time.Now().UTC().Truncate(time.Millisecond)
				graceEnd := lateAt.Add(3 * 24 * time.Hour)
				s.Require().NoError(repo.ApplyTransition(ctx, found.ID(), valueobjects.StatusPastDue, graceEnd, lateAt))
				updated, err := repo.FindByOrderID(ctx, orderID)
				s.Require().NoError(err)
				s.Assert().Equal(valueobjects.StatusPastDue, updated.Status())
				s.Assert().WithinDuration(graceEnd, updated.GraceEnd(), time.Second)
			},
		},
		{
			name: "deve persistir e retornar o last event at",
			expect: func(ctx context.Context, repo interfaces.SubscriptionRepository) {
				plan := s.newPlan(valueobjects.PlanCodeAnnual, 365)
				ft := s.newFunnelToken("token-lasteventat-001")
				sub := entities.NewSubscription(plan, ft)
				occurredAt := time.Now().UTC().Truncate(time.Millisecond)
				s.Require().NoError(sub.Activate(occurredAt))
				orderID := "order-lasteventat-001"
				s.Require().NoError(repo.UpsertByOrder(ctx, interfaces.UpsertByOrderParams{OrderID: orderID, Subscription: sub, PeriodStart: occurredAt}))
				found, err := repo.FindByOrderID(ctx, orderID)
				s.Require().NoError(err)
				s.Assert().WithinDuration(occurredAt, found.LastEventAt(), time.Second)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			repo := s.newRepo()
			scenario.expect(ctx, repo)
		})
	}
}

type RF17ConcurrentSubSuite struct {
	suite.Suite
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func TestRF17ConcurrentSubSuite(t *testing.T) {
	suite.Run(t, new(RF17ConcurrentSubSuite))
}

func (s *RF17ConcurrentSubSuite) SetupTest() {}

func (s *RF17ConcurrentSubSuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *RF17ConcurrentSubSuite) TestRF17_SecondActiveSubscriptionForSameUserFails() {
	scenarios := []struct {
		name string
	}{
		{name: "deve falhar ao vincular segunda assinatura ativa ao mesmo usuario"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			dbtx := s.mgr.DBTX(ctx)
			repo := s.factory.SubscriptionRepository(dbtx)
			userID := "550e8400-e29b-41d4-a716-446655440001"

			plan, err := valueobjects.NewPlan("MONTHLY", 30)
			s.Require().NoError(err)
			ft1, err := valueobjects.NewFunnelToken("token-rf17-user-001")
			s.Require().NoError(err)
			ft2, err := valueobjects.NewFunnelToken("token-rf17-user-002")
			s.Require().NoError(err)

			sub1 := entities.NewSubscription(plan, ft1)
			occurredAt := time.Now().UTC()
			s.Require().NoError(sub1.Activate(occurredAt))
			orderID1 := "order-rf17-001"
			s.Require().NoError(repo.UpsertByOrder(ctx, interfaces.UpsertByOrderParams{OrderID: orderID1, Subscription: sub1, PeriodStart: occurredAt}))
			found1, err := repo.FindByOrderID(ctx, orderID1)
			s.Require().NoError(err)
			s.Require().NoError(repo.BindUser(ctx, found1.ID(), userID))

			sub2 := entities.NewSubscription(plan, ft2)
			s.Require().NoError(sub2.Activate(occurredAt.Add(time.Second)))
			orderID2 := "order-rf17-002"
			s.Require().NoError(repo.UpsertByOrder(ctx, interfaces.UpsertByOrderParams{OrderID: orderID2, Subscription: sub2, PeriodStart: occurredAt.Add(time.Second)}))
			found2, err := repo.FindByOrderID(ctx, orderID2)
			s.Require().NoError(err)

			err = repo.BindUser(ctx, found2.ID(), userID)
			s.Require().Error(err)
			s.Assert().True(errors.Is(err, billingpostgres.ErrConcurrentActiveSub), "RF-17: expected ErrConcurrentActiveSub, got: %v", err)
		})
	}
}
