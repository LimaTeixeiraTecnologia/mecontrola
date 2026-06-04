package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type SubscriptionStatusSuite struct {
	suite.Suite
}

func TestSubscriptionStatus(t *testing.T) {
	suite.Run(t, new(SubscriptionStatusSuite))
}

func (s *SubscriptionStatusSuite) TestZeroValueIsUnknown() {
	var st valueobjects.SubscriptionStatus
	s.Equal("UNKNOWN", st.String())
}

func (s *SubscriptionStatusSuite) TestString() {
	cases := []struct {
		status   valueobjects.SubscriptionStatus
		expected string
	}{
		{valueobjects.SubscriptionStatusTrialing, "TRIALING"},
		{valueobjects.SubscriptionStatusActive, "ACTIVE"},
		{valueobjects.SubscriptionStatusPastDue, "PAST_DUE"},
		{valueobjects.SubscriptionStatusCanceledPending, "CANCELED_PENDING"},
		{valueobjects.SubscriptionStatusExpired, "EXPIRED"},
		{valueobjects.SubscriptionStatusRefunded, "REFUNDED"},
		{valueobjects.SubscriptionStatusUnknown, "UNKNOWN"},
	}

	for _, tc := range cases {
		s.Run(tc.expected, func() {
			s.Equal(tc.expected, tc.status.String())
		})
	}
}

func (s *SubscriptionStatusSuite) TestIsCreatable() {
	cases := []struct {
		status    valueobjects.SubscriptionStatus
		creatable bool
	}{
		{valueobjects.SubscriptionStatusActive, true},
		{valueobjects.SubscriptionStatusTrialing, true},
		{valueobjects.SubscriptionStatusPastDue, false},
		{valueobjects.SubscriptionStatusCanceledPending, false},
		{valueobjects.SubscriptionStatusExpired, false},
		{valueobjects.SubscriptionStatusRefunded, false},
		{valueobjects.SubscriptionStatusUnknown, false},
	}

	for _, tc := range cases {
		s.Run(tc.status.String(), func() {
			s.Equal(tc.creatable, tc.status.IsCreatable())
		})
	}
}
