package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type UserStatusSuite struct {
	suite.Suite
}

func TestUserStatus(t *testing.T) {
	suite.Run(t, new(UserStatusSuite))
}

func (s *UserStatusSuite) TestString() {
	cases := []struct {
		status   valueobjects.UserStatus
		expected string
	}{
		{valueobjects.UserStatusUnknown, "UNKNOWN"},
		{valueobjects.UserStatusActive, "ACTIVE"},
		{valueobjects.UserStatusBlocked, "BLOCKED"},
		{valueobjects.UserStatusDeleted, "DELETED"},
	}

	for _, tc := range cases {
		s.Run(tc.expected, func() {
			s.Equal(tc.expected, tc.status.String())
		})
	}
}

func (s *UserStatusSuite) TestParseUserStatus() {
	cases := []struct {
		input    string
		expected valueobjects.UserStatus
		ok       bool
	}{
		{input: "ACTIVE", expected: valueobjects.UserStatusActive, ok: true},
		{input: "BLOCKED", expected: valueobjects.UserStatusBlocked, ok: true},
		{input: "DELETED", expected: valueobjects.UserStatusDeleted, ok: true},
		{input: "UNKNOWN", expected: valueobjects.UserStatusUnknown, ok: false},
		{input: "invalid", expected: valueobjects.UserStatusUnknown, ok: false},
		{input: "", expected: valueobjects.UserStatusUnknown, ok: false},
	}

	for _, tc := range cases {
		s.Run(tc.input, func() {
			got, ok := valueobjects.ParseUserStatus(tc.input)
			s.Equal(tc.ok, ok)
			s.Equal(tc.expected, got)
		})
	}
}

func (s *UserStatusSuite) TestZeroValue() {
	var status valueobjects.UserStatus
	s.Equal(valueobjects.UserStatusUnknown, status)
	s.Equal("UNKNOWN", status.String())
}

func (s *UserStatusSuite) TestIota() {
	// garante que zero-value é UserStatusUnknown (R5.8)
	s.Equal(valueobjects.UserStatus(0), valueobjects.UserStatusUnknown)
	s.Equal(valueobjects.UserStatus(1), valueobjects.UserStatusActive)
	s.Equal(valueobjects.UserStatus(2), valueobjects.UserStatusBlocked)
	s.Equal(valueobjects.UserStatus(3), valueobjects.UserStatusDeleted)
}
