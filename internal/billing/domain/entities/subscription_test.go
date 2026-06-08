package entities_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type SubscriptionSuite struct {
	suite.Suite
}

func TestSubscriptionSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionSuite))
}

func (s *SubscriptionSuite) SetupTest() {}

func (s *SubscriptionSuite) TestSubscriptionTransitions() {
	type args struct {
		setup  func(entities.Subscription) entities.Subscription
		expect func(entities.Subscription, error)
	}

	scenarios := []struct {
		name string
		args args
	}{
		{
			name: "deve ativar assinatura e definir period end a partir do occurred at",
			args: args{
				setup: func(subscription entities.Subscription) entities.Subscription {
					err := subscription.Activate(time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC))
					s.Require().NoError(err)
					return subscription
				},
				expect: func(subscription entities.Subscription, err error) {
					s.Require().NoError(err)
					occurredAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
					s.Equal(valueobjects.StatusActive, subscription.Status())
					s.Equal(occurredAt.Add(30*24*time.Hour), subscription.PeriodEnd())
					s.True(subscription.GraceEnd().IsZero())
					s.Equal(occurredAt, subscription.LastEventAt())
				},
			},
		},
		{
			name: "deve renovar a partir do fim do periodo quando assinatura ainda estiver ativa",
			args: args{
				setup: func(subscription entities.Subscription) entities.Subscription {
					initialAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
					s.Require().NoError(subscription.Activate(initialAt))
					err := subscription.Renew(initialAt.Add(10 * 24 * time.Hour))
					s.Require().NoError(err)
					return subscription
				},
				expect: func(subscription entities.Subscription, err error) {
					s.Require().NoError(err)
					initialAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
					renewedAt := initialAt.Add(10 * 24 * time.Hour)
					s.Equal(valueobjects.StatusActive, subscription.Status())
					s.Equal(initialAt.Add(60*24*time.Hour), subscription.PeriodEnd())
					s.Equal(renewedAt, subscription.LastEventAt())
				},
			},
		},
		{
			name: "deve reativar canceled pending e reiniciar periodo quando assinatura estiver expirada",
			args: args{
				setup: func(subscription entities.Subscription) entities.Subscription {
					initialAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
					s.Require().NoError(subscription.Activate(initialAt))
					s.Require().NoError(subscription.MarkCanceled(initialAt.Add(5 * 24 * time.Hour)))
					err := subscription.Renew(initialAt.Add(45 * 24 * time.Hour))
					s.Require().NoError(err)
					return subscription
				},
				expect: func(subscription entities.Subscription, err error) {
					s.Require().NoError(err)
					renewedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).Add(45 * 24 * time.Hour)
					s.Equal(valueobjects.StatusActive, subscription.Status())
					s.Equal(renewedAt.Add(30*24*time.Hour), subscription.PeriodEnd())
					s.True(subscription.GraceEnd().IsZero())
				},
			},
		},
		{
			name: "deve marcar past due e definir janela de grace",
			args: args{
				setup: func(subscription entities.Subscription) entities.Subscription {
					activatedAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
					s.Require().NoError(subscription.Activate(activatedAt))
					err := subscription.MarkPastDue(activatedAt.Add(31*24*time.Hour), 3*24*time.Hour)
					s.Require().NoError(err)
					return subscription
				},
				expect: func(subscription entities.Subscription, err error) {
					s.Require().NoError(err)
					lateAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC).Add(31 * 24 * time.Hour)
					s.Equal(valueobjects.StatusPastDue, subscription.Status())
					s.Equal(lateAt.Add(3*24*time.Hour), subscription.GraceEnd())
					s.Equal(lateAt, subscription.LastEventAt())
				},
			},
		},
		{
			name: "deve marcar canceled pending preservando period end",
			args: args{
				setup: func(subscription entities.Subscription) entities.Subscription {
					activatedAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
					s.Require().NoError(subscription.Activate(activatedAt))
					expectedPeriodEnd := subscription.PeriodEnd()
					err := subscription.MarkCanceled(activatedAt.Add(5 * 24 * time.Hour))
					s.Require().NoError(err)
					s.Equal(expectedPeriodEnd, subscription.PeriodEnd())
					return subscription
				},
				expect: func(subscription entities.Subscription, err error) {
					s.Require().NoError(err)
					s.Equal(valueobjects.StatusCanceledPending, subscription.Status())
					s.True(subscription.GraceEnd().IsZero())
				},
			},
		},
		{
			name: "deve marcar refunded como terminal e limpar grace",
			args: args{
				setup: func(subscription entities.Subscription) entities.Subscription {
					activatedAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
					s.Require().NoError(subscription.Activate(activatedAt))
					s.Require().NoError(subscription.MarkPastDue(activatedAt.Add(31*24*time.Hour), 3*24*time.Hour))
					err := subscription.MarkRefunded(activatedAt.Add(32 * 24 * time.Hour))
					s.Require().NoError(err)
					return subscription
				},
				expect: func(subscription entities.Subscription, err error) {
					refundedAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC).Add(32 * 24 * time.Hour)
					s.Require().NoError(err)
					s.Equal(valueobjects.StatusRefunded, subscription.Status())
					s.True(subscription.GraceEnd().IsZero())
					s.Equal(refundedAt, subscription.LastEventAt())
					s.Require().ErrorIs(subscription.MarkCanceled(refundedAt.Add(24*time.Hour)), entities.ErrTransitionNotAllowed)
				},
			},
		},
		{
			name: "deve rejeitar occurred at zero em todos os metodos",
			args: args{
				expect: func(subscription entities.Subscription, err error) {
					s.Require().NoError(err)
					assert.ErrorIs(s.T(), subscription.Activate(time.Time{}), entities.ErrOccurredAtRequired)
					assert.ErrorIs(s.T(), subscription.Renew(time.Time{}), entities.ErrOccurredAtRequired)
					assert.ErrorIs(s.T(), subscription.MarkPastDue(time.Time{}, 3*24*time.Hour), entities.ErrOccurredAtRequired)
					assert.ErrorIs(s.T(), subscription.MarkCanceled(time.Time{}), entities.ErrOccurredAtRequired)
					assert.ErrorIs(s.T(), subscription.MarkRefunded(time.Time{}), entities.ErrOccurredAtRequired)
				},
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			subscription := newSubscription(s.T())
			if scenario.args.setup != nil {
				subscription = scenario.args.setup(subscription)
			}
			scenario.args.expect(subscription, nil)
		})
	}
}

func newSubscription(t *testing.T) entities.Subscription {
	t.Helper()

	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	require.NoError(t, err)
	token, err := valueobjects.NewFunnelToken("funnel-token")
	require.NoError(t, err)

	return entities.NewSubscription(plan, token)
}
