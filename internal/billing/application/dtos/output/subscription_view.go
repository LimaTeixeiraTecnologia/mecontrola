package output

import "time"

type SubscriptionView struct {
	ID          string
	OrderID     string
	UserID      string
	FunnelToken string
	PlanCode    string
	Status      string
	PeriodStart time.Time
	PeriodEnd   time.Time
	GraceEnd    time.Time
	LastEventAt time.Time
}
