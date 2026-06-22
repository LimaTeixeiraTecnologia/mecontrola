package input

import "errors"

type ProjectAuthEvent struct {
	EventType string
	Payload   []byte
}

func (i *ProjectAuthEvent) Validate() error {
	var errs []error
	if i.EventType == "" {
		errs = append(errs, ErrEventTypeRequired)
	}
	if len(i.Payload) == 0 {
		errs = append(errs, ErrPayloadRequired)
	}
	return errors.Join(errs...)
}
