package output

import "time"

const MonthlyEntryKindTransaction = "transaction"
const MonthlyEntryKindCardInvoiceItem = "card_invoice_item"

type MonthlyEntry struct {
	Kind                    string    `json:"kind"`
	ID                      string    `json:"id"`
	UserID                  string    `json:"user_id"`
	RefMonth                string    `json:"ref_month"`
	AmountCents             int64     `json:"amount_cents"`
	Direction               string    `json:"direction"`
	Description             string    `json:"description,omitempty"`
	CategoryID              string    `json:"category_id,omitempty"`
	SubcategoryID           *string   `json:"subcategory_id,omitempty"`
	CategoryNameSnapshot    string    `json:"category_name_snapshot,omitempty"`
	SubcategoryNameSnapshot string    `json:"subcategory_name_snapshot,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
}
