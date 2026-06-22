package input

import (
	"encoding/json"
	"errors"
)

type ProjectSubscriptionEvent struct {
	EventType string
	Payload   json.RawMessage
}

func (i *ProjectSubscriptionEvent) Validate() error {
	var errs []error
	if i.EventType == "" {
		errs = append(errs, ErrEventTypeRequired)
	}
	if len(i.Payload) == 0 {
		errs = append(errs, ErrPayloadRequired)
	}
	return errors.Join(errs...)
}
