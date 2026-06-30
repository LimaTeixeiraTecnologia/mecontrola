package input

import "errors"

type ActivateFromInboundInput struct {
	PeerE164  string
	Text      string
	MessageID string
}

func (i *ActivateFromInboundInput) Validate() error {
	var errs []error
	if i.PeerE164 == "" {
		errs = append(errs, ErrPeerE164Required)
	}
	if i.MessageID == "" {
		errs = append(errs, ErrMessageIDRequired)
	}
	return errors.Join(errs...)
}
