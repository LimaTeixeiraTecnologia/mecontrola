package input

import "time"

type ProcessRefundOrChargebackInput struct {
	EnvelopeID string
	SaleID     string
	OrderID    string
	Trigger    string
	OccurredAt time.Time
}
