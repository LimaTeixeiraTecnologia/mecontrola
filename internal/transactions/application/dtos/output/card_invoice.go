package output

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type CardInvoiceItemOutput struct {
	ID               uuid.UUID `json:"id"`
	InvoiceID        uuid.UUID `json:"invoice_id"`
	RefMonth         string    `json:"ref_month"`
	InstallmentIndex int       `json:"installment_index"`
	AmountCents      int64     `json:"amount_cents"`
}

type CardInvoice struct {
	ID              uuid.UUID               `json:"id"`
	UserID          uuid.UUID               `json:"user_id"`
	CardID          uuid.UUID               `json:"card_id"`
	RefMonth        string                  `json:"ref_month"`
	ClosingAt       time.Time               `json:"closing_at"`
	DueAt           time.Time               `json:"due_at"`
	ItemsTotalCents int64                   `json:"items_total_cents"`
	Version         int64                   `json:"version"`
	Items           []CardInvoiceItemOutput `json:"items,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

func CardInvoiceFrom(inv *entities.CardInvoice, items []*entities.CardInvoiceItem) CardInvoice {
	out := CardInvoice{
		ID:              inv.ID(),
		UserID:          inv.UserID().UUID(),
		CardID:          inv.CardID().UUID(),
		RefMonth:        inv.RefMonth().String(),
		ClosingAt:       inv.ClosingAt(),
		DueAt:           inv.DueAt(),
		ItemsTotalCents: inv.ItemsTotalCents(),
		Version:         inv.Version(),
		CreatedAt:       inv.CreatedAt(),
		UpdatedAt:       inv.UpdatedAt(),
	}
	for _, item := range items {
		out.Items = append(out.Items, CardInvoiceItemOutput{
			ID:               item.ID(),
			InvoiceID:        item.InvoiceID(),
			RefMonth:         item.RefMonth().String(),
			InstallmentIndex: item.InstallmentIndex(),
			AmountCents:      item.Amount().Cents(),
		})
	}
	return out
}
