package input

import (
	"errors"
	"time"
)

type ProcessRefundOrChargebackInput struct {
	EnvelopeID string
	SaleID     string
	OrderID    string
	Trigger    string
	OccurredAt time.Time
}

func (i *ProcessRefundOrChargebackInput) Validate() error {
	var errs []error
	if i.SaleID == "" {
		errs = append(errs, ErrSaleIDRequired)
	}
	if i.OrderID == "" {
		errs = append(errs, ErrOrderIDRequired)
	}
	if i.OccurredAt.IsZero() {
		errs = append(errs, ErrOccurredAtRequired)
	}
	return errors.Join(errs...)
}
