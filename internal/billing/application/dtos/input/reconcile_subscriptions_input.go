package input

import (
	"errors"
	"time"
)

type ReconcileSubscriptionsInput struct {
	WindowStart time.Time
	WindowEnd   time.Time
}

func (i *ReconcileSubscriptionsInput) Validate() error {
	var errs []error
	if i.WindowStart.IsZero() {
		errs = append(errs, ErrWindowStartRequired)
	}
	if i.WindowEnd.IsZero() {
		errs = append(errs, ErrWindowEndRequired)
	}
	if !i.WindowStart.IsZero() && !i.WindowEnd.IsZero() && !i.WindowEnd.After(i.WindowStart) {
		errs = append(errs, ErrWindowEndBeforeStart)
	}
	return errors.Join(errs...)
}
