package input

import "time"

type HandlePaidWithoutTokenInput struct {
	ExternalSaleID     string
	CustomerMobileE164 string
	CustomerEmail      string
	PaidAt             time.Time
}
