package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type CanonicalEventTypeSuite struct {
	suite.Suite
}

func TestCanonicalEventType(t *testing.T) {
	suite.Run(t, new(CanonicalEventTypeSuite))
}

func (s *CanonicalEventTypeSuite) TestZeroValueIsUnknown() {
	var e valueobjects.CanonicalEventType
	s.Equal("UNKNOWN", e.String())
}

func (s *CanonicalEventTypeSuite) TestString() {
	cases := []struct {
		evt      valueobjects.CanonicalEventType
		expected string
	}{
		{valueobjects.CanonicalEventPurchaseApproved, "purchase_approved"},
		{valueobjects.CanonicalEventRenewed, "renewed"},
		{valueobjects.CanonicalEventLate, "late"},
		{valueobjects.CanonicalEventCanceled, "canceled"},
		{valueobjects.CanonicalEventRefunded, "refunded"},
		{valueobjects.CanonicalEventChargeback, "chargeback"},
		{valueobjects.CanonicalEventUnknown, "UNKNOWN"},
	}

	for _, tc := range cases {
		s.Run(tc.expected, func() {
			s.Equal(tc.expected, tc.evt.String())
		})
	}
}
