package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
)

type stubSub struct {
	status         domain.SubscriptionStatus
	periodEnd      time.Time
	gracePeriodEnd time.Time
}

func (s stubSub) Status() domain.SubscriptionStatus { return s.status }
func (s stubSub) PeriodEnd() time.Time              { return s.periodEnd }
func (s stubSub) GracePeriodEnd() time.Time         { return s.gracePeriodEnd }

func TestIsEntitled(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name         string
		sub          domain.Subscription
		wantEntitled bool
		wantReason   domain.Reason
	}{
		{
			name:         "nil subscription",
			sub:          nil,
			wantEntitled: false,
			wantReason:   domain.ReasonNoSubscription,
		},
		{
			name:         "ACTIVE period_end > now",
			sub:          stubSub{status: domain.SubscriptionActive, periodEnd: future},
			wantEntitled: true,
			wantReason:   domain.ReasonActive,
		},
		{
			name:         "ACTIVE period_end <= now",
			sub:          stubSub{status: domain.SubscriptionActive, periodEnd: past},
			wantEntitled: false,
			wantReason:   domain.ReasonExpired,
		},
		{
			name:         "TRIALING period_end > now",
			sub:          stubSub{status: domain.SubscriptionTrialing, periodEnd: future},
			wantEntitled: true,
			wantReason:   domain.ReasonTrialing,
		},
		{
			name:         "TRIALING period_end <= now",
			sub:          stubSub{status: domain.SubscriptionTrialing, periodEnd: past},
			wantEntitled: false,
			wantReason:   domain.ReasonExpired,
		},
		{
			name:         "PAST_DUE grace_period_end > now",
			sub:          stubSub{status: domain.SubscriptionPastDue, gracePeriodEnd: future},
			wantEntitled: true,
			wantReason:   domain.ReasonPastDueGrace,
		},
		{
			name:         "PAST_DUE grace_period_end <= now",
			sub:          stubSub{status: domain.SubscriptionPastDue, gracePeriodEnd: past},
			wantEntitled: false,
			wantReason:   domain.ReasonPastDueNoGrace,
		},
		{
			name:         "PAST_DUE grace_period_end zero",
			sub:          stubSub{status: domain.SubscriptionPastDue},
			wantEntitled: false,
			wantReason:   domain.ReasonPastDueNoGrace,
		},
		{
			name:         "CANCELED_PENDING period_end > now",
			sub:          stubSub{status: domain.SubscriptionCanceledPending, periodEnd: future},
			wantEntitled: true,
			wantReason:   domain.ReasonCanceledPending,
		},
		{
			name:         "CANCELED_PENDING period_end <= now",
			sub:          stubSub{status: domain.SubscriptionCanceledPending, periodEnd: past},
			wantEntitled: false,
			wantReason:   domain.ReasonExpired,
		},
		{
			name:         "EXPIRED",
			sub:          stubSub{status: domain.SubscriptionExpired},
			wantEntitled: false,
			wantReason:   domain.ReasonExpired,
		},
		{
			name:         "REFUNDED",
			sub:          stubSub{status: domain.SubscriptionRefunded},
			wantEntitled: false,
			wantReason:   domain.ReasonRefunded,
		},
		{
			name:         "status desconhecido",
			sub:          stubSub{status: "UNKNOWN_STATUS"},
			wantEntitled: false,
			wantReason:   domain.ReasonExpired,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entitled, reason := domain.IsEntitled(tc.sub, now)
			assert.Equal(t, tc.wantEntitled, entitled)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}
