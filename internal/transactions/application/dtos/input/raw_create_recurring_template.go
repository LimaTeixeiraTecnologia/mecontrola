package input

import "github.com/google/uuid"

type RawCreateRecurringTemplate struct {
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
}
