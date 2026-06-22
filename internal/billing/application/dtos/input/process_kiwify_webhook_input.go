package input

import "errors"

type ProcessKiwifyWebhookInput struct {
	RawBody         []byte
	SignatureStatus string
}

func (i *ProcessKiwifyWebhookInput) Validate() error {
	var errs []error
	if len(i.RawBody) == 0 {
		errs = append(errs, ErrRawBodyRequired)
	}
	if i.SignatureStatus == "" {
		errs = append(errs, ErrSignatureStatusRequired)
	}
	return errors.Join(errs...)
}
