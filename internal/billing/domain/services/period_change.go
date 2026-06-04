package services

import "time"

type PeriodChange struct {
	NewStart time.Time
	NewEnd   time.Time
}

func (p PeriodChange) AdvancesPeriod() bool { return !p.NewStart.IsZero() && !p.NewEnd.IsZero() }

func NoPeriodChange() PeriodChange { return PeriodChange{} }
