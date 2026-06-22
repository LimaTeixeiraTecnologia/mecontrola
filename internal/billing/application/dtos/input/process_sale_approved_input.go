package input

import (
	"errors"
	"time"
)

type ProcessSaleApprovedInput struct {
	EnvelopeID         string
	SaleID             string
	KiwifyProductID    string
	OrderID            string
	KiwifySubID        string
	FunnelToken        string
	CustomerMobileE164 string
	CustomerEmail      string
	OccurredAt         time.Time
}

func (i *ProcessSaleApprovedInput) Validate() error {
	var errs []error
	if i.SaleID == "" {
		errs = append(errs, ErrSaleIDRequired)
	}
	if i.KiwifyProductID == "" {
		errs = append(errs, ErrKiwifyProductIDRequired)
	}
	if i.OrderID == "" {
		errs = append(errs, ErrOrderIDRequired)
	}
	if i.KiwifySubID == "" {
		errs = append(errs, ErrKiwifySubIDRequired)
	}
	if i.OccurredAt.IsZero() {
		errs = append(errs, ErrOccurredAtRequired)
	}
	return errors.Join(errs...)
}
