package input

import "github.com/google/uuid"

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
