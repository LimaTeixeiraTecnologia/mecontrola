package output

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type Transaction struct {
	ID                      uuid.UUID  `json:"id"`
	UserID                  uuid.UUID  `json:"user_id"`
	Direction               string     `json:"direction"`
	PaymentMethod           string     `json:"payment_method"`
	AmountCents             int64      `json:"amount_cents"`
	Description             string     `json:"description"`
	CategoryID              uuid.UUID  `json:"category_id"`
	SubcategoryID           *uuid.UUID `json:"subcategory_id,omitempty"`
	CategoryNameSnapshot    string     `json:"category_name_snapshot"`
	SubcategoryNameSnapshot string     `json:"subcategory_name_snapshot,omitempty"`
	RefMonth                string     `json:"ref_month"`
	OccurredAt              time.Time  `json:"occurred_at"`
	Version                 int64      `json:"version"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	Reconciled              bool       `json:"reconciled,omitempty"`

	CardID            *uuid.UUID `json:"card_id,omitempty"`
	InstallmentsTotal int        `json:"installments_total,omitempty"`

	CategoryOutcome     string  `json:"category_outcome,omitempty"`
	CategoryScore       float64 `json:"category_score,omitempty"`
	CategoryConfidence  string  `json:"category_confidence,omitempty"`
	CategoryQuality     string  `json:"category_quality,omitempty"`
	CategorySignalType  string  `json:"category_signal_type,omitempty"`
	CategoryMatchedTerm string  `json:"category_matched_term,omitempty"`
	CategoryMatchReason string  `json:"category_match_reason,omitempty"`
}

func TransactionFrom(t *entities.Transaction) Transaction {
	out := Transaction{
		ID:                      t.ID(),
		UserID:                  t.UserID().UUID(),
		Direction:               t.Direction().String(),
		PaymentMethod:           t.PaymentMethod().String(),
		AmountCents:             t.Amount().Cents(),
		Description:             t.Description().String(),
		CategoryID:              t.CategoryID().UUID(),
		CategoryNameSnapshot:    t.CategoryNameSnapshot(),
		SubcategoryNameSnapshot: t.SubcategoryNameSnapshot(),
		RefMonth:                t.RefMonth().String(),
		OccurredAt:              t.OccurredAt(),
		Version:                 t.Version(),
		CreatedAt:               t.CreatedAt(),
		UpdatedAt:               t.UpdatedAt(),

		CategoryOutcome:     t.Evidence().Outcome(),
		CategoryScore:       t.Evidence().Score(),
		CategoryConfidence:  t.Evidence().Confidence(),
		CategoryQuality:     t.Evidence().Quality(),
		CategorySignalType:  t.Evidence().SignalType(),
		CategoryMatchedTerm: t.Evidence().MatchedTerm(),
		CategoryMatchReason: t.Evidence().MatchReason(),
	}
	if sub, ok := t.SubcategoryID().Get(); ok {
		v := sub.UUID()
		out.SubcategoryID = &v
	}
	if card, ok := t.CardID().Get(); ok {
		v := card.UUID()
		out.CardID = &v
	}
	if inst, ok := t.InstallmentsTotal().Get(); ok {
		out.InstallmentsTotal = inst.Value()
	}
	return out
}
