package input

import "errors"

type EstablishPrincipalInput struct {
	WhatsAppNumber string
	RequestID      string
	ClientIPRaw    string
}

func (i *EstablishPrincipalInput) Validate() error {
	var errs []error
	if i.WhatsAppNumber == "" {
		errs = append(errs, ErrWhatsAppNumberRequired)
	}
	return errors.Join(errs...)
}
