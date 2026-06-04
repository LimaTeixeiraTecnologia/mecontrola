package valueobjects_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type BillingPeriodSuite struct {
	suite.Suite
}

func TestBillingPeriod(t *testing.T) {
	suite.Run(t, new(BillingPeriodSuite))
}

func (s *BillingPeriodSuite) TestNewBillingPeriodFor() {
	cases := []struct {
		name        string
		code        valueobjects.PlanCode
		expected    time.Duration
		expectedErr error
	}{
		{name: "Monthly 30 dias", code: valueobjects.PlanCodeMonthly, expected: 30 * 24 * time.Hour},
		{name: "Quarterly 90 dias", code: valueobjects.PlanCodeQuarterly, expected: 90 * 24 * time.Hour},
		{name: "Annual 365 dias", code: valueobjects.PlanCodeAnnual, expected: 365 * 24 * time.Hour},
		{name: "Unknown retorna erro", code: valueobjects.PlanCodeUnknown, expectedErr: valueobjects.ErrUnknownPlanCode},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.NewBillingPeriodFor(tc.code)
			if tc.expectedErr != nil {
				s.True(errors.Is(err, tc.expectedErr))
				s.True(got.IsZero())
			} else {
				s.NoError(err)
				s.Equal(tc.expected, got.Length())
			}
		})
	}
}

func (s *BillingPeriodSuite) TestAnnualLength() {
	period, err := valueobjects.NewBillingPeriodFor(valueobjects.PlanCodeAnnual)
	s.NoError(err)
	s.Equal(365*24*time.Hour, period.Length())
}

func (s *BillingPeriodSuite) TestAdvanceEqualsLength() {
	cases := []valueobjects.PlanCode{
		valueobjects.PlanCodeMonthly,
		valueobjects.PlanCodeQuarterly,
		valueobjects.PlanCodeAnnual,
	}

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, code := range cases {
		s.Run(code.String(), func() {
			period, err := valueobjects.NewBillingPeriodFor(code)
			s.NoError(err)
			advanced := period.Advance(base)
			s.Equal(period.Length(), advanced.Sub(base))
		})
	}
}

func (s *BillingPeriodSuite) TestIsZero() {
	var p valueobjects.BillingPeriod
	s.True(p.IsZero())

	period, _ := valueobjects.NewBillingPeriodFor(valueobjects.PlanCodeMonthly)
	s.False(period.IsZero())
}
