package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type TransitionReasonSuite struct {
	suite.Suite
}

func TestTransitionReason(t *testing.T) {
	suite.Run(t, new(TransitionReasonSuite))
}

func (s *TransitionReasonSuite) TestZeroValueIsUnknown() {
	var r valueobjects.TransitionReason
	s.Equal("UNKNOWN", r.String())
}

func (s *TransitionReasonSuite) TestString() {
	cases := []struct {
		reason   valueobjects.TransitionReason
		expected string
	}{
		{valueobjects.TransitionReasonPurchaseApproved, "purchase_approved"},
		{valueobjects.TransitionReasonRenewed, "renewed"},
		{valueobjects.TransitionReasonLate, "late"},
		{valueobjects.TransitionReasonCanceled, "canceled"},
		{valueobjects.TransitionReasonRefunded, "refunded"},
		{valueobjects.TransitionReasonChargebackReceived, "chargeback_received"},
		{valueobjects.TransitionReasonReconciliationSync, "reconciliation_sync"},
		{valueobjects.TransitionReasonUnknown, "UNKNOWN"},
	}

	for _, tc := range cases {
		s.Run(tc.expected, func() {
			s.Equal(tc.expected, tc.reason.String())
		})
	}
}
