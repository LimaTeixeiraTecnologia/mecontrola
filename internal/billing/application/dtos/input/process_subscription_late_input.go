package input

import "time"

type ProcessSubscriptionLateInput struct {
	EnvelopeID  string
	SaleID      string
	OrderID     string
	KiwifySubID string
	OccurredAt  time.Time
}
