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

type CardPurchase struct {
	ID                      uuid.UUID               `json:"id"`
	UserID                  uuid.UUID               `json:"user_id"`
	CardID                  uuid.UUID               `json:"card_id"`
	TotalAmountCents        int64                   `json:"total_amount_cents"`
	InstallmentsTotal       int                     `json:"installments_total"`
	Description             string                  `json:"description"`
	CategoryID              uuid.UUID               `json:"category_id"`
	SubcategoryID           *uuid.UUID              `json:"subcategory_id,omitempty"`
	CategoryNameSnapshot    string                  `json:"category_name_snapshot"`
	SubcategoryNameSnapshot string                  `json:"subcategory_name_snapshot,omitempty"`
	PurchasedAt             time.Time               `json:"purchased_at"`
	Version                 int64                   `json:"version"`
	RefMonthsAffected       []string                `json:"ref_months_affected,omitempty"`
	Items                   []CardInvoiceItemOutput `json:"items,omitempty"`
	CreatedAt               time.Time               `json:"created_at"`
	UpdatedAt               time.Time               `json:"updated_at"`
}

func CardPurchaseFrom(p *entities.CardPurchase, items []*entities.CardInvoiceItem, refMonthsAffected []string) CardPurchase {
	out := CardPurchase{
		ID:                      p.ID(),
		UserID:                  p.UserID().UUID(),
		CardID:                  p.CardID().UUID(),
		TotalAmountCents:        p.TotalAmount().Cents(),
		InstallmentsTotal:       p.InstallmentsTotal().Value(),
		Description:             p.Description().String(),
		CategoryID:              p.CategoryID().UUID(),
		CategoryNameSnapshot:    p.CategoryNameSnapshot(),
		SubcategoryNameSnapshot: p.SubcategoryNameSnapshot(),
		PurchasedAt:             p.PurchasedAt(),
		Version:                 p.Version(),
		RefMonthsAffected:       refMonthsAffected,
		CreatedAt:               p.CreatedAt(),
		UpdatedAt:               p.UpdatedAt(),
	}
	if sub, ok := p.SubcategoryID().Get(); ok {
		v := sub.UUID()
		out.SubcategoryID = &v
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
