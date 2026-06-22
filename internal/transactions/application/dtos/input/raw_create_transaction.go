package input

import (
	"errors"

	"github.com/google/uuid"
)

type RawCreateTransaction struct {
	Direction     string     `json:"direction"`
	PaymentMethod string     `json:"payment_method"`
	AmountCents   int64      `json:"amount_cents"`
	Description   string     `json:"description"`
	CategoryID    uuid.UUID  `json:"category_id"`
	SubcategoryID *uuid.UUID `json:"subcategory_id,omitempty"`
	OccurredAt    string     `json:"occurred_at"`
}

func (i *RawCreateTransaction) Validate() error {
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
	if i.OccurredAt == "" {
		errs = append(errs, ErrInputOccurredAtRequired)
	}
	return errors.Join(errs...)
}
