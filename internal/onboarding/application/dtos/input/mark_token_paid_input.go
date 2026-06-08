package input

import "time"

type MarkTokenPaidInput struct {
	SubscriptionID     string
	FunnelToken        string
	CustomerMobileE164 string
	CustomerEmail      string
	ExternalSaleID     string
	PaidAt             time.Time
}
