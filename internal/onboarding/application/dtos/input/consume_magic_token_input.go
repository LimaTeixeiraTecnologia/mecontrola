package input

import (
	"errors"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type ConsumeMagicTokenInput struct {
	Token          string
	FromE164       string
	ActivationPath valueobjects.ActivationPath
}

func (i *ConsumeMagicTokenInput) Validate() error {
	var errs []error
	if i.Token == "" {
		errs = append(errs, ErrTokenRequired)
	}
	if i.FromE164 == "" {
		errs = append(errs, ErrFromE164Required)
	}
	if string(i.ActivationPath) == "" {
		errs = append(errs, ErrActivationPathRequired)
	}
	return errors.Join(errs...)
}
