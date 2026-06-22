package input

import "errors"

type FindUserByWhatsApp struct {
	WhatsAppNumber string
}

func (i *FindUserByWhatsApp) Validate() error {
	var errs []error
	if i.WhatsAppNumber == "" {
		errs = append(errs, ErrWhatsAppNumberRequired)
	}
	return errors.Join(errs...)
}
