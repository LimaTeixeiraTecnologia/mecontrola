package services_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
)

type CanonicalEventSuite struct {
	suite.Suite
}

func TestCanonicalEventSuite(t *testing.T) {
	suite.Run(t, new(CanonicalEventSuite))
}

func (s *CanonicalEventSuite) TestZeroValueIsSafe() {
	var ce services.CanonicalEvent
	s.Equal("", ce.ExternalEventID)
	s.Equal("", ce.ExternalSubscriptionID)
	s.Equal("", ce.SignupToken)
	s.Equal(int64(0), ce.RefundAmountCents)
	s.True(ce.OccurredAt.IsZero())
	s.True(ce.PeriodStart.IsZero())
	s.True(ce.PeriodEnd.IsZero())
	s.True(ce.Customer.WhatsApp.IsZero())
	s.Equal("", ce.Customer.Email)
}

func (s *CanonicalEventSuite) TestCanonicalSubscriptionZeroValueIsSafe() {
	var cs services.CanonicalSubscription
	s.Equal("", cs.ExternalID)
	s.True(cs.PeriodStart.IsZero())
	s.True(cs.PeriodEnd.IsZero())
}
