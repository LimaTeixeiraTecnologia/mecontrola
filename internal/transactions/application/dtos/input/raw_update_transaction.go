package input

import (
	"errors"

	"github.com/google/uuid"
)

type RawUpdateTransaction struct {
	Direction     string     `json:"direction"`
	PaymentMethod string     `json:"payment_method"`
	AmountCents   int64      `json:"amount_cents"`
	Description   string     `json:"description"`
	CategoryID    uuid.UUID  `json:"category_id"`
	SubcategoryID *uuid.UUID `json:"subcategory_id,omitempty"`
	OccurredAt    string     `json:"occurred_at"`
	Version       int64      `json:"version"`
}

func (i *RawUpdateTransaction) Validate() error {
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
	if i.Version <= 0 {
		errs = append(errs, ErrInputVersionRequired)
	}
	return errors.Join(errs...)
}
