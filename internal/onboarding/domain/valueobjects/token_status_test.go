package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type TokenStatusSuite struct {
	suite.Suite
}

func TestTokenStatusSuite(t *testing.T) {
	suite.Run(t, new(TokenStatusSuite))
}

func (s *TokenStatusSuite) TestParseTokenStatus_ValidStatuses() {
	cases := []struct {
		raw      string
		expected valueobjects.TokenStatus
	}{
		{"PENDING", valueobjects.TokenStatusPending},
		{"PAID", valueobjects.TokenStatusPaid},
		{"CONSUMED", valueobjects.TokenStatusConsumed},
		{"EXPIRED", valueobjects.TokenStatusExpired},
	}

	for _, tc := range cases {
		status, err := valueobjects.ParseTokenStatus(tc.raw)
		s.Require().NoError(err)
		s.Equal(tc.expected, status)
		s.Equal(tc.raw, status.String())
	}
}

func (s *TokenStatusSuite) TestParseTokenStatus_InvalidStatusReturnsError() {
	_, err := valueobjects.ParseTokenStatus("INVALID")
	s.ErrorIs(err, valueobjects.ErrTokenStatusInvalid)
}

func (s *TokenStatusSuite) TestTokenStatus_ZeroValueIsUnknown() {
	var s2 valueobjects.TokenStatus
	s.Equal("UNKNOWN", s2.String())
}
