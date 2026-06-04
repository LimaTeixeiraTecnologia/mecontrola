package services_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type StateMachineSuite struct {
	suite.Suite
	sm services.StateMachine
}

func TestStateMachineSuite(t *testing.T) {
	suite.Run(t, new(StateMachineSuite))
}

func (s *StateMachineSuite) SetupTest() {
	s.sm = services.NewStateMachine()
}

func (s *StateMachineSuite) TestAssertLegal() {
	type row struct {
		from  valueobjects.SubscriptionStatus
		to    valueobjects.SubscriptionStatus
		legal bool
	}

	unknown := valueobjects.SubscriptionStatusUnknown
	trialing := valueobjects.SubscriptionStatusTrialing
	active := valueobjects.SubscriptionStatusActive
	pastDue := valueobjects.SubscriptionStatusPastDue
	canceled := valueobjects.SubscriptionStatusCanceledPending
	expired := valueobjects.SubscriptionStatusExpired
	refunded := valueobjects.SubscriptionStatusRefunded

	allStatuses := []valueobjects.SubscriptionStatus{unknown, trialing, active, pastDue, canceled, expired, refunded}

	legalSet := map[[2]valueobjects.SubscriptionStatus]bool{
		{trialing, active}:   true,
		{trialing, expired}:  true,
		{active, pastDue}:    true,
		{active, canceled}:   true,
		{active, refunded}:   true,
		{pastDue, active}:    true,
		{pastDue, expired}:   true,
		{pastDue, refunded}:  true,
		{canceled, expired}:  true,
		{canceled, active}:   true,
		{canceled, refunded}: true,
	}

	rows := make([]row, 0, len(allStatuses)*len(allStatuses))
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			key := [2]valueobjects.SubscriptionStatus{from, to}
			rows = append(rows, row{from: from, to: to, legal: legalSet[key]})
		}
	}

	for _, tc := range rows {
		s.Run(tc.from.String()+"->"+tc.to.String(), func() {
			err := s.sm.AssertLegal(tc.from, tc.to)
			if tc.legal {
				s.NoError(err)
			} else {
				s.ErrorIs(err, services.ErrIllegalTransition)
			}
		})
	}
}

func (s *StateMachineSuite) TestAssertLegalActiveToExpiredIsIllegal() {
	err := s.sm.AssertLegal(valueobjects.SubscriptionStatusActive, valueobjects.SubscriptionStatusExpired)
	s.ErrorIs(err, services.ErrIllegalTransition)
}

func (s *StateMachineSuite) TestAssertLegalExpiredToActiveIsIllegal() {
	err := s.sm.AssertLegal(valueobjects.SubscriptionStatusExpired, valueobjects.SubscriptionStatusActive)
	s.ErrorIs(err, services.ErrIllegalTransition)
}

func (s *StateMachineSuite) TestAssertLegalRefundedToAnyIsIllegal() {
	allStatuses := []valueobjects.SubscriptionStatus{
		valueobjects.SubscriptionStatusUnknown,
		valueobjects.SubscriptionStatusTrialing,
		valueobjects.SubscriptionStatusActive,
		valueobjects.SubscriptionStatusPastDue,
		valueobjects.SubscriptionStatusCanceledPending,
		valueobjects.SubscriptionStatusExpired,
		valueobjects.SubscriptionStatusRefunded,
	}
	for _, to := range allStatuses {
		s.Run("refunded->"+to.String(), func() {
			err := s.sm.AssertLegal(valueobjects.SubscriptionStatusRefunded, to)
			s.ErrorIs(err, services.ErrIllegalTransition)
		})
	}
}

func (s *StateMachineSuite) TestDefaultGracePeriod() {
	s.Equal(services.DefaultGracePeriod.Hours(), float64(7*24))
}
