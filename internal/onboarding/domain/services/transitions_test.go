package services_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type TransitionSuite struct {
	suite.Suite
	svc services.TransitionService
}

func TestTransitionSuite(t *testing.T) {
	suite.Run(t, new(TransitionSuite))
}

func (s *TransitionSuite) SetupTest() {
	s.svc = services.NewTransitionService()
}

func (s *TransitionSuite) TestCanMarkPaid_OnlyFromPending() {
	s.True(s.svc.CanMarkPaid(valueobjects.TokenStatusPending))
	s.False(s.svc.CanMarkPaid(valueobjects.TokenStatusPaid))
	s.False(s.svc.CanMarkPaid(valueobjects.TokenStatusConsumed))
	s.False(s.svc.CanMarkPaid(valueobjects.TokenStatusExpired))
}

func (s *TransitionSuite) TestCanConsume_OnlyFromPaid() {
	s.True(s.svc.CanConsume(valueobjects.TokenStatusPaid))
	s.False(s.svc.CanConsume(valueobjects.TokenStatusPending))
	s.False(s.svc.CanConsume(valueobjects.TokenStatusConsumed))
	s.False(s.svc.CanConsume(valueobjects.TokenStatusExpired))
}

func (s *TransitionSuite) TestCanMarkOutreach_OnlyFromPaid() {
	s.True(s.svc.CanMarkOutreach(valueobjects.TokenStatusPaid))
	s.False(s.svc.CanMarkOutreach(valueobjects.TokenStatusPending))
	s.False(s.svc.CanMarkOutreach(valueobjects.TokenStatusConsumed))
	s.False(s.svc.CanMarkOutreach(valueobjects.TokenStatusExpired))
}

func (s *TransitionSuite) TestCanExpire_PendingAndPaid() {
	s.True(s.svc.CanExpire(valueobjects.TokenStatusPending))
	s.True(s.svc.CanExpire(valueobjects.TokenStatusPaid))
	s.False(s.svc.CanExpire(valueobjects.TokenStatusConsumed))
	s.False(s.svc.CanExpire(valueobjects.TokenStatusExpired))
}

func (s *TransitionSuite) TestValidateConsume_TableDriven() {
	cases := []struct {
		status      valueobjects.TokenStatus
		expectedErr error
	}{
		{valueobjects.TokenStatusPaid, nil},
		{valueobjects.TokenStatusPending, domain.ErrTokenNotYetPaid},
		{valueobjects.TokenStatusExpired, domain.ErrTokenExpired},
		{valueobjects.TokenStatusConsumed, domain.ErrTokenAlreadyConsumedSame},
	}

	for _, tc := range cases {
		err := s.svc.ValidateConsume(tc.status)
		if tc.expectedErr == nil {
			s.NoError(err)
		} else {
			s.ErrorIs(err, tc.expectedErr)
		}
	}
}
