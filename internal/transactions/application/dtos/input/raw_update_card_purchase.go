package input

import "github.com/google/uuid"

type RawUpdateCardPurchase struct {
	TotalAmountCents  int64      `json:"total_amount_cents"`
	InstallmentsTotal int        `json:"installments_total"`
	Description       string     `json:"description"`
	CategoryID        uuid.UUID  `json:"category_id"`
	SubcategoryID     *uuid.UUID `json:"subcategory_id,omitempty"`
	PurchasedAt       string     `json:"purchased_at"`
	Version           int64      `json:"version"`
}
