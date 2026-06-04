package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type PlanCodeSuite struct {
	suite.Suite
}

func TestPlanCode(t *testing.T) {
	suite.Run(t, new(PlanCodeSuite))
}

func (s *PlanCodeSuite) TestZeroValueIsUnknown() {
	var p valueobjects.PlanCode
	s.Equal("UNKNOWN", p.String())
}

func (s *PlanCodeSuite) TestParsePlanCode() {
	cases := []struct {
		name        string
		input       string
		expected    valueobjects.PlanCode
		expectedErr error
	}{
		{name: "MONTHLY", input: "MONTHLY", expected: valueobjects.PlanCodeMonthly},
		{name: "QUARTERLY", input: "QUARTERLY", expected: valueobjects.PlanCodeQuarterly},
		{name: "ANNUAL", input: "ANNUAL", expected: valueobjects.PlanCodeAnnual},
		{name: "desconhecido", input: "XYZ", expected: valueobjects.PlanCodeUnknown, expectedErr: valueobjects.ErrUnknownPlanCode},
		{name: "vazio", input: "", expected: valueobjects.PlanCodeUnknown, expectedErr: valueobjects.ErrUnknownPlanCode},
		{name: "minusculo", input: "monthly", expected: valueobjects.PlanCodeUnknown, expectedErr: valueobjects.ErrUnknownPlanCode},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.ParsePlanCode(tc.input)
			if tc.expectedErr != nil {
				s.True(errors.Is(err, tc.expectedErr), "esperado %v, got %v", tc.expectedErr, err)
				s.Equal(tc.expected, got)
			} else {
				s.NoError(err)
				s.Equal(tc.expected, got)
			}
		})
	}
}

func (s *PlanCodeSuite) TestString() {
	cases := []struct {
		code     valueobjects.PlanCode
		expected string
	}{
		{valueobjects.PlanCodeMonthly, "MONTHLY"},
		{valueobjects.PlanCodeQuarterly, "QUARTERLY"},
		{valueobjects.PlanCodeAnnual, "ANNUAL"},
		{valueobjects.PlanCodeUnknown, "UNKNOWN"},
	}

	for _, tc := range cases {
		s.Run(tc.expected, func() {
			s.Equal(tc.expected, tc.code.String())
		})
	}
}
