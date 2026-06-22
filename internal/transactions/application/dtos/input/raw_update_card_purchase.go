package input

import (
	"errors"

	"github.com/google/uuid"
)

type RawUpdateCardPurchase struct {
	TotalAmountCents  int64      `json:"total_amount_cents"`
	InstallmentsTotal int        `json:"installments_total"`
	Description       string     `json:"description"`
	CategoryID        uuid.UUID  `json:"category_id"`
	SubcategoryID     *uuid.UUID `json:"subcategory_id,omitempty"`
	PurchasedAt       string     `json:"purchased_at"`
	Version           int64      `json:"version"`
}

func (i *RawUpdateCardPurchase) Validate() error {
	var errs []error
	if i.TotalAmountCents <= 0 {
		errs = append(errs, ErrInputTotalAmountRequired)
	}
	if i.InstallmentsTotal < 1 || i.InstallmentsTotal > 24 {
		errs = append(errs, ErrInputInstallmentsOutOfRange)
	}
	if i.Description == "" {
		errs = append(errs, ErrInputDescriptionRequired)
	}
	if i.CategoryID == uuid.Nil {
		errs = append(errs, ErrInputCategoryIDRequired)
	}
	if i.PurchasedAt == "" {
		errs = append(errs, ErrInputPurchasedAtRequired)
	}
	if i.Version <= 0 {
		errs = append(errs, ErrInputVersionRequired)
	}
	return errors.Join(errs...)
}
