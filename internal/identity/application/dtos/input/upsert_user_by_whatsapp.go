package input

import "errors"

type UpsertUserByWhatsApp struct {
	WhatsAppNumber string
	Email          string
	DisplayName    string
}

func (i *UpsertUserByWhatsApp) Validate() error {
	var errs []error
	if i.WhatsAppNumber == "" {
		errs = append(errs, ErrWhatsAppNumberRequired)
	}
	if i.DisplayName == "" {
		errs = append(errs, ErrDisplayNameRequired)
	}
	return errors.Join(errs...)
}
