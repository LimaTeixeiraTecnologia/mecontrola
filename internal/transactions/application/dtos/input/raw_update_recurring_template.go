package input

import (
	"errors"

	"github.com/google/uuid"
)

type RawUpdateRecurringTemplate struct {
	Direction         string     `json:"direction"`
	PaymentMethod     string     `json:"payment_method"`
	CardID            *uuid.UUID `json:"card_id,omitempty"`
	AmountCents       int64      `json:"amount_cents"`
	Description       string     `json:"description"`
	CategoryID        uuid.UUID  `json:"category_id"`
	SubcategoryID     *uuid.UUID `json:"subcategory_id,omitempty"`
	Frequency         string     `json:"frequency"`
	DayOfMonth        int        `json:"day_of_month"`
	InstallmentsTotal int        `json:"installments_total"`
	StartedAt         string     `json:"started_at"`
	EndedAt           *string    `json:"ended_at,omitempty"`
	Version           int64      `json:"version"`
}

func (i *RawUpdateRecurringTemplate) Validate() error {
	var errs []error
	if i.Direction == "" {
		errs = append(errs, ErrInputDirectionRequired)
	}
	if i.PaymentMethod == "" {
		errs = append(errs, ErrInputPaymentMethodRequired)
	}
	if i.AmountCents <= 0 {
		errs = append(errs, ErrInputAmountCentsRequired)
	}
	if i.Description == "" {
		errs = append(errs, ErrInputDescriptionRequired)
	}
	if i.CategoryID == uuid.Nil {
		errs = append(errs, ErrInputCategoryIDRequired)
	}
	if i.Frequency == "" {
		errs = append(errs, ErrInputFrequencyRequired)
	}
	if i.DayOfMonth < 1 || i.DayOfMonth > 28 {
		errs = append(errs, ErrInputDayOfMonthOutOfRange)
	}
	if i.StartedAt == "" {
		errs = append(errs, ErrInputStartedAtRequired)
	}
	if i.Version <= 0 {
		errs = append(errs, ErrInputVersionRequired)
	}
	return errors.Join(errs...)
}
