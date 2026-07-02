package input

import "errors"

type BestPurchaseDay struct {
	Bank   string
	DueDay int
}

func (i *BestPurchaseDay) Validate() error {
	var errs []error
	if i.Bank == "" {
		errs = append(errs, ErrCardBankRequired)
	}
	if i.DueDay < 1 || i.DueDay > 31 {
		errs = append(errs, ErrCardDueDayInvalid)
	}
	return errors.Join(errs...)
}
