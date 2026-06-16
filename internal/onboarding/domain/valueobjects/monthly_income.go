package valueobjects

import (
	"errors"
	"fmt"
)

var (
	ErrMonthlyIncomeBelowMinimum = errors.New("onboarding: monthly income below minimum")
	ErrMonthlyIncomeAboveMaximum = errors.New("onboarding: monthly income above maximum")
)

const (
	monthlyIncomeMinCents int64 = 50000
	monthlyIncomeMaxCents int64 = 10000000000
)

type MonthlyIncome struct {
	cents int64
}

func NewMonthlyIncome(cents int64) (MonthlyIncome, error) {
	if cents < monthlyIncomeMinCents {
		return MonthlyIncome{}, fmt.Errorf("onboarding: %d: %w", cents, ErrMonthlyIncomeBelowMinimum)
	}
	if cents > monthlyIncomeMaxCents {
		return MonthlyIncome{}, fmt.Errorf("onboarding: %d: %w", cents, ErrMonthlyIncomeAboveMaximum)
	}
	return MonthlyIncome{cents: cents}, nil
}

func (m MonthlyIncome) Cents() int64 {
	return m.cents
}

func (m MonthlyIncome) Equal(other MonthlyIncome) bool {
	return m.cents == other.cents
}
