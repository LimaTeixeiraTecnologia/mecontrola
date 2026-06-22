package input

import (
	"errors"
	"time"
)

type ProcessSubscriptionRenewedInput struct {
	EnvelopeID      string
	SaleID          string
	KiwifyProductID string
	OrderID         string
	KiwifySubID     string
	OccurredAt      time.Time
}

func (i *ProcessSubscriptionRenewedInput) Validate() error {
	var errs []error
	if i.KiwifySubID == "" {
		errs = append(errs, ErrKiwifySubIDRequired)
	}
	if i.OccurredAt.IsZero() {
		errs = append(errs, ErrOccurredAtRequired)
	}
	return errors.Join(errs...)
}
