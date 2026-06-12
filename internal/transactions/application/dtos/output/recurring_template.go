package output

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type RecurringTemplate struct {
	ID                      uuid.UUID  `json:"id"`
	UserID                  uuid.UUID  `json:"user_id"`
	Direction               string     `json:"direction"`
	PaymentMethod           string     `json:"payment_method"`
	CardID                  *uuid.UUID `json:"card_id,omitempty"`
	AmountCents             int64      `json:"amount_cents"`
	Description             string     `json:"description"`
	CategoryID              uuid.UUID  `json:"category_id"`
	SubcategoryID           *uuid.UUID `json:"subcategory_id,omitempty"`
	CategoryNameSnapshot    string     `json:"category_name_snapshot"`
	SubcategoryNameSnapshot string     `json:"subcategory_name_snapshot,omitempty"`
	Frequency               string     `json:"frequency"`
	DayOfMonth              int        `json:"day_of_month"`
	InstallmentsTotal       int        `json:"installments_total"`
	StartedAt               time.Time  `json:"started_at"`
	EndedAt                 *time.Time `json:"ended_at,omitempty"`
	Version                 int64      `json:"version"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

func RecurringTemplateFrom(t *entities.RecurringTemplate) RecurringTemplate {
	out := RecurringTemplate{
		ID:                      t.ID(),
		UserID:                  t.UserID().UUID(),
		Direction:               t.Direction().String(),
		PaymentMethod:           t.PaymentMethod().String(),
		AmountCents:             t.Amount().Cents(),
		Description:             t.Description().String(),
		CategoryID:              t.CategoryID().UUID(),
		CategoryNameSnapshot:    t.CategoryNameSnapshot(),
		SubcategoryNameSnapshot: t.SubcategoryNameSnapshot(),
		Frequency:               t.Frequency().String(),
		DayOfMonth:              t.DayOfMonth().Value(),
		InstallmentsTotal:       t.InstallmentsTotal().Value(),
		StartedAt:               t.StartedAt(),
		Version:                 t.Version(),
		CreatedAt:               t.CreatedAt(),
		UpdatedAt:               t.UpdatedAt(),
	}
	if cid, ok := t.CardID().Get(); ok {
		v := cid.UUID()
		out.CardID = &v
	}
	if sub, ok := t.SubcategoryID().Get(); ok {
		v := sub.UUID()
		out.SubcategoryID = &v
	}
	if ea, ok := t.EndedAt().Get(); ok {
		out.EndedAt = &ea
	}
	return out
}
