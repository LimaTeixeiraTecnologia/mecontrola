package input

import (
	"errors"
	"time"
)

type MarkTokenPaidInput struct {
	SubscriptionID     string
	FunnelToken        string
	CustomerMobileE164 string
	CustomerEmail      string
	ExternalSaleID     string
	PaidAt             time.Time
}

func (i *MarkTokenPaidInput) Validate() error {
	var errs []error
	if i.SubscriptionID == "" {
		errs = append(errs, ErrSubscriptionIDRequired)
	}
	if i.FunnelToken == "" {
		errs = append(errs, ErrFunnelTokenRequired)
	}
	if i.ExternalSaleID == "" {
		errs = append(errs, ErrExternalSaleIDRequired)
	}
	if i.PaidAt.IsZero() {
		errs = append(errs, ErrPaidAtRequired)
	}
	return errors.Join(errs...)
}
