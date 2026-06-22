package input

import (
	"errors"
	"time"
)

type ProcessSubscriptionCanceledInput struct {
	EnvelopeID  string
	SaleID      string
	OrderID     string
	KiwifySubID string
	OccurredAt  time.Time
}

func (i *ProcessSubscriptionCanceledInput) Validate() error {
	var errs []error
	if i.KiwifySubID == "" {
		errs = append(errs, ErrKiwifySubIDRequired)
	}
	if i.OccurredAt.IsZero() {
		errs = append(errs, ErrOccurredAtRequired)
	}
	return errors.Join(errs...)
}
