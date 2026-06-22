package input

import "errors"

type FindUserByID struct {
	ID string
}

func (i *FindUserByID) Validate() error {
	var errs []error
	if i.ID == "" {
		errs = append(errs, ErrIDRequired)
	}
	return errors.Join(errs...)
}
