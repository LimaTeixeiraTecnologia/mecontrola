package input

import (
	"errors"

	"github.com/google/uuid"
)

type RawUpdateTransaction struct {
	Direction           string     `json:"direction"`
	PaymentMethod       string     `json:"payment_method"`
	AmountCents         int64      `json:"amount_cents"`
	Description         string     `json:"description"`
	CategoryID          uuid.UUID  `json:"category_id"`
	SubcategoryID       *uuid.UUID `json:"subcategory_id,omitempty"`
	CardID              *uuid.UUID `json:"card_id,omitempty"`
	Installments        int        `json:"installments,omitempty"`
	OccurredAt          string     `json:"occurred_at"`
	Version             int64      `json:"version"`
	CategorySource      string     `json:"category_source,omitempty"`
	CategoryOutcome     string     `json:"category_outcome,omitempty"`
	CategoryScore       float64    `json:"category_score,omitempty"`
	CategoryConfidence  string     `json:"category_confidence,omitempty"`
	CategoryQuality     string     `json:"category_quality,omitempty"`
	CategorySignalType  string     `json:"category_signal_type,omitempty"`
	CategoryMatchedTerm string     `json:"category_matched_term,omitempty"`
	CategoryMatchReason string     `json:"category_match_reason,omitempty"`
	CategoryVersion     int64      `json:"category_version,omitempty"`
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
