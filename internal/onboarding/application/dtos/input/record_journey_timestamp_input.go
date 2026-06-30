package input

import "errors"

const (
	JourneyEventPageOpened     = "page_opened"
	JourneyEventWhatsAppOpened = "whatsapp_opened"
)

type RecordJourneyTimestampInput struct {
	ClearToken string
	Event      string
}

func (i *RecordJourneyTimestampInput) Validate() error {
	var errs []error
	if i.ClearToken == "" {
		errs = append(errs, ErrTokenRequired)
	}
	if i.Event == "" {
		errs = append(errs, ErrEventRequired)
	} else if i.Event != JourneyEventPageOpened && i.Event != JourneyEventWhatsAppOpened {
		errs = append(errs, ErrEventInvalid)
	}
	return errors.Join(errs...)
}
