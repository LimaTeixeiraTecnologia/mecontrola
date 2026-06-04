package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
)

type fakeSubscription struct {
	status           services.SubscriptionStatus
	currentPeriodEnd time.Time
	gracePeriodEnd   time.Time
}

func (f *fakeSubscription) Status() services.SubscriptionStatus { return f.status }
func (f *fakeSubscription) CurrentPeriodEnd() time.Time         { return f.currentPeriodEnd }
func (f *fakeSubscription) GracePeriodEnd() time.Time           { return f.gracePeriodEnd }

type EntitlementSuite struct {
	suite.Suite
	checker services.EntitlementChecker
}

func (s *EntitlementSuite) SetupSuite() {
	s.checker = services.NewEntitlementChecker()
}

func (s *EntitlementSuite) TestIsEntitled() {
	base := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	future := base.Add(24 * time.Hour)
	past := base.Add(-24 * time.Hour)

	cases := []struct {
		name     string
		sub      services.Subscription
		now      time.Time
		expected bool
	}{
		{
			name:     "nil subscription returns false",
			sub:      nil,
			now:      base,
			expected: false,
		},
		{
			name: "TRIALING now before CurrentPeriodEnd returns true",
			sub: &fakeSubscription{
				status:           services.StatusTrialing,
				currentPeriodEnd: future,
			},
			now:      base,
			expected: true,
		},
		{
			name: "TRIALING now after CurrentPeriodEnd returns false",
			sub: &fakeSubscription{
				status:           services.StatusTrialing,
				currentPeriodEnd: past,
			},
			now:      base,
			expected: false,
		},
		{
			name: "TRIALING boundary now == CurrentPeriodEnd returns false",
			sub: &fakeSubscription{
				status:           services.StatusTrialing,
				currentPeriodEnd: base,
			},
			now:      base,
			expected: false,
		},
		{
			name: "ACTIVE now before CurrentPeriodEnd returns true",
			sub: &fakeSubscription{
				status:           services.StatusActive,
				currentPeriodEnd: future,
			},
			now:      base,
			expected: true,
		},
		{
			name: "ACTIVE now after CurrentPeriodEnd returns false",
			sub: &fakeSubscription{
				status:           services.StatusActive,
				currentPeriodEnd: past,
			},
			now:      base,
			expected: false,
		},
		{
			name: "PAST_DUE now before GracePeriodEnd returns true",
			sub: &fakeSubscription{
				status:         services.StatusPastDue,
				gracePeriodEnd: future,
			},
			now:      base,
			expected: true,
		},
		{
			name: "PAST_DUE now after GracePeriodEnd returns false",
			sub: &fakeSubscription{
				status:         services.StatusPastDue,
				gracePeriodEnd: past,
			},
			now:      base,
			expected: false,
		},
		{
			name: "PAST_DUE boundary now == GracePeriodEnd returns false",
			sub: &fakeSubscription{
				status:         services.StatusPastDue,
				gracePeriodEnd: base,
			},
			now:      base,
			expected: false,
		},
		{
			name: "CANCELED_PENDING now before GracePeriodEnd returns true",
			sub: &fakeSubscription{
				status:         services.StatusCanceledPending,
				gracePeriodEnd: future,
			},
			now:      base,
			expected: true,
		},
		{
			name: "CANCELED_PENDING now after GracePeriodEnd returns false",
			sub: &fakeSubscription{
				status:         services.StatusCanceledPending,
				gracePeriodEnd: past,
			},
			now:      base,
			expected: false,
		},
		{
			name: "EXPIRED always returns false",
			sub: &fakeSubscription{
				status:           services.StatusExpired,
				currentPeriodEnd: future,
				gracePeriodEnd:   future,
			},
			now:      base,
			expected: false,
		},
		{
			name: "REFUNDED always returns false",
			sub: &fakeSubscription{
				status:           services.StatusRefunded,
				currentPeriodEnd: future,
				gracePeriodEnd:   future,
			},
			now:      base,
			expected: false,
		},
		{
			name: "StatusUnknown always returns false",
			sub: &fakeSubscription{
				status:           services.StatusUnknown,
				currentPeriodEnd: future,
				gracePeriodEnd:   future,
			},
			now:      base,
			expected: false,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			result := s.checker.IsEntitled(tc.sub, tc.now)
			s.Equal(tc.expected, result)
		})
	}
}

func (s *EntitlementSuite) TestSubscriptionStatusString() {
	cases := []struct {
		status   services.SubscriptionStatus
		expected string
	}{
		{services.StatusUnknown, "UNKNOWN"},
		{services.StatusTrialing, "TRIALING"},
		{services.StatusActive, "ACTIVE"},
		{services.StatusPastDue, "PAST_DUE"},
		{services.StatusCanceledPending, "CANCELED_PENDING"},
		{services.StatusExpired, "EXPIRED"},
		{services.StatusRefunded, "REFUNDED"},
	}

	for _, tc := range cases {
		s.Equal(tc.expected, tc.status.String())
	}
}

func TestEntitlementSuite(t *testing.T) {
	suite.Run(t, new(EntitlementSuite))
}
