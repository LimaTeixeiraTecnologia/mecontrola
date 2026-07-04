package input

import (
	"errors"

	"github.com/google/uuid"
)

type RawCreateTransaction struct {
	Direction       string     `json:"direction"`
	PaymentMethod   string     `json:"payment_method"`
	AmountCents     int64      `json:"amount_cents"`
	Description     string     `json:"description"`
	CategoryID      uuid.UUID  `json:"category_id"`
	SubcategoryID   *uuid.UUID `json:"subcategory_id,omitempty"`
	CardID          *uuid.UUID `json:"card_id,omitempty"`
	Installments    int        `json:"installments,omitempty"`
	OccurredAt      string     `json:"occurred_at"`
	OriginWamid     string     `json:"origin_wamid,omitempty"`
	OriginItemSeq   int        `json:"origin_item_seq,omitempty"`
	OriginOperation string     `json:"origin_operation,omitempty"`
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
	if i.OriginWamid != "" && i.OriginOperation == "" {
		errs = append(errs, errors.New("origin_operation: obrigatório quando origin_wamid presente"))
	}
	if i.PaymentMethod == "credit_card" {
		if i.CardID == nil {
			errs = append(errs, ErrInputCardIDRequired)
		}
		if i.Installments < 1 || i.Installments > 24 {
			errs = append(errs, ErrInputInstallmentsOutOfRange)
		}
	}
	return errors.Join(errs...)
}
