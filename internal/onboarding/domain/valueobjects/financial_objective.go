package valueobjects

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrFinancialObjectiveEmpty   = errors.New("onboarding: financial objective required")
	ErrFinancialObjectiveTooLong = errors.New("onboarding: financial objective too long")
)

const financialObjectiveMaxLen = 280

type FinancialObjective struct {
	text string
}

func NewFinancialObjective(raw string) (FinancialObjective, error) {
	v := strings.Join(strings.Fields(raw), " ")
	if v == "" {
		return FinancialObjective{}, ErrFinancialObjectiveEmpty
	}
	if length := len([]rune(v)); length > financialObjectiveMaxLen {
		return FinancialObjective{}, fmt.Errorf("onboarding: len=%d max=%d: %w", length, financialObjectiveMaxLen, ErrFinancialObjectiveTooLong)
	}
	return FinancialObjective{text: v}, nil
}

func (o FinancialObjective) String() string {
	return o.text
}
