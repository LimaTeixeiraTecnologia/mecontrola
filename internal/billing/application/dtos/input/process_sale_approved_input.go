package input

import "time"

type ProcessSaleApprovedInput struct {
	EnvelopeID      string
	SaleID          string
	KiwifyProductID string
	OrderID         string
	FunnelToken     string
	OccurredAt      time.Time
}
