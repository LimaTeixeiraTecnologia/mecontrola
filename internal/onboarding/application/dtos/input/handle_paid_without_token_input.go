package input

import (
	"errors"
	"time"
)

type HandlePaidWithoutTokenInput struct {
	ExternalSaleID     string
	CustomerMobileE164 string
	CustomerEmail      string
	PaidAt             time.Time
}

func (i *HandlePaidWithoutTokenInput) Validate() error {
	var errs []error
	if i.ExternalSaleID == "" {
		errs = append(errs, ErrExternalSaleIDRequired)
	}
	if i.PaidAt.IsZero() {
		errs = append(errs, ErrPaidAtRequired)
	}
	return errors.Join(errs...)
}
