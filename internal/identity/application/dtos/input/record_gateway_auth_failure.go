package input

import "errors"

type RecordGatewayAuthFailureInput struct {
	UserIDRaw   string
	Reason      string
	RequestID   string
	ClientIPRaw string
}

func (i *RecordGatewayAuthFailureInput) Validate() error {
	var errs []error
	if i.Reason == "" {
		errs = append(errs, ErrReasonRequired)
	}
	return errors.Join(errs...)
}
