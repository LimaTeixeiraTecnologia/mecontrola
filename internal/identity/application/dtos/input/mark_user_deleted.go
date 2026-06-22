package input

import "errors"

type MarkUserDeleted struct {
	ID string
}

func (i *MarkUserDeleted) Validate() error {
	var errs []error
	if i.ID == "" {
		errs = append(errs, ErrIDRequired)
	}
	return errors.Join(errs...)
}
