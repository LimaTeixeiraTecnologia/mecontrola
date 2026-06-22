package input

import "errors"

type AnonymizeUserAuthEvents struct {
	Payload []byte
}

func (i *AnonymizeUserAuthEvents) Validate() error {
	var errs []error
	if len(i.Payload) == 0 {
		errs = append(errs, ErrPayloadRequired)
	}
	return errors.Join(errs...)
}
