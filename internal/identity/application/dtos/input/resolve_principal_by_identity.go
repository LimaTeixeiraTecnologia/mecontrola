package input

import "errors"

type ResolvePrincipalByIdentity struct {
	Channel     string
	ExternalID  string
	RequestID   string
	ClientIPRaw string
}

func (i *ResolvePrincipalByIdentity) Validate() error {
	var errs []error
	if i.Channel == "" {
		errs = append(errs, ErrChannelRequired)
	}
	if i.ExternalID == "" {
		errs = append(errs, ErrExternalIDRequired)
	}
	return errors.Join(errs...)
}
