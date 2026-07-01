package input

import (
	"errors"

	"github.com/google/uuid"
)

type RawCreateCardPurchase struct {
	CardID            uuid.UUID  `json:"card_id"`
	TotalAmountCents  int64      `json:"total_amount_cents"`
	InstallmentsTotal int        `json:"installments_total"`
	Description       string     `json:"description"`
	CategoryID        uuid.UUID  `json:"category_id"`
	SubcategoryID     *uuid.UUID `json:"subcategory_id,omitempty"`
	PurchasedAt       string     `json:"purchased_at"`
	OriginWamid       string     `json:"origin_wamid,omitempty"`
	OriginItemSeq     int        `json:"origin_item_seq,omitempty"`
	OriginOperation   string     `json:"origin_operation,omitempty"`
}

func (i *RawCreateCardPurchase) Validate() error {
	var errs []error
	if i.CardID == uuid.Nil {
		errs = append(errs, ErrInputCardIDRequired)
	}
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
	if i.OriginWamid != "" && i.OriginOperation == "" {
		errs = append(errs, errors.New("origin_operation: obrigatório quando origin_wamid presente"))
	}
	return errors.Join(errs...)
}
