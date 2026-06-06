package input

import "time"

type ProcessSubscriptionRenewedInput struct {
	EnvelopeID      string
	SaleID          string
	KiwifyProductID string
	OrderID         string
	KiwifySubID     string
	OccurredAt      time.Time
}
