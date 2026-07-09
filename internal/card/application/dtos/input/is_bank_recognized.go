package input

import "errors"

type IsBankRecognized struct {
	Bank string
}

func (i *IsBankRecognized) Validate() error {
	var errs []error
	if i.Bank == "" {
		errs = append(errs, ErrCardBankRequired)
	}
	return errors.Join(errs...)
}
