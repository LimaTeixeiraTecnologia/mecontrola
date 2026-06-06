package input

import "time"

type ProcessSubscriptionCanceledInput struct {
	EnvelopeID  string
	SaleID      string
	OrderID     string
	KiwifySubID string
	OccurredAt  time.Time
}
