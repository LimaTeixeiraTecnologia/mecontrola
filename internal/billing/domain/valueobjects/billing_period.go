package valueobjects

import "time"

type BillingPeriod struct{ length time.Duration }

func NewBillingPeriodFor(code PlanCode) (BillingPeriod, error) {
	switch code {
	case PlanCodeMonthly:
		return BillingPeriod{length: 30 * 24 * time.Hour}, nil
	case PlanCodeQuarterly:
		return BillingPeriod{length: 90 * 24 * time.Hour}, nil
	case PlanCodeAnnual:
		return BillingPeriod{length: 365 * 24 * time.Hour}, nil
	default:
		return BillingPeriod{}, ErrUnknownPlanCode
	}
}

func (p BillingPeriod) Advance(from time.Time) time.Time { return from.Add(p.length) }
func (p BillingPeriod) Length() time.Duration            { return p.length }
func (p BillingPeriod) IsZero() bool                     { return p.length == 0 }
