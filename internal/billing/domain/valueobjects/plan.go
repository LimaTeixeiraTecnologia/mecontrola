package valueobjects

import (
	"errors"
	"fmt"
	"time"
)

var ErrPlanCodeInvalid = errors.New("billing: invalid plan code")

var ErrPlanDurationInvalid = errors.New("billing: invalid plan duration")

type PlanCode string

const (
	PlanCodeMonthly   PlanCode = "MONTHLY"
	PlanCodeQuarterly PlanCode = "QUARTERLY"
	PlanCodeAnnual    PlanCode = "ANNUAL"
)

type Plan struct {
	code         PlanCode
	durationDays int
}

func NewPlan(code string, durationDays int) (Plan, error) {
	planCode := PlanCode(code)
	if !planCode.IsSupported() {
		return Plan{}, fmt.Errorf("billing: %q: %w", code, ErrPlanCodeInvalid)
	}
	if durationDays <= 0 {
		return Plan{}, fmt.Errorf("billing: %d: %w", durationDays, ErrPlanDurationInvalid)
	}

	return Plan{
		code:         planCode,
		durationDays: durationDays,
	}, nil
}

func (c PlanCode) IsSupported() bool {
	switch c {
	case PlanCodeMonthly, PlanCodeQuarterly, PlanCodeAnnual:
		return true
	default:
		return false
	}
}

func (p Plan) Code() PlanCode {
	return p.code
}

func (p Plan) Duration() time.Duration {
	return time.Duration(p.durationDays) * 24 * time.Hour
}

func (p Plan) DurationDays() int {
	return p.durationDays
}
