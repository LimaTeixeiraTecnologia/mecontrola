package entities

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type TransactionCreated struct {
	EventID       uuid.UUID                  `json:"event_id"`
	AggregateID   uuid.UUID                  `json:"aggregate_id"`
	UserID        uuid.UUID                  `json:"user_id"`
	OccurredAt    time.Time                  `json:"occurred_at"`
	Direction     valueobjects.Direction     `json:"direction"`
	PaymentMethod valueobjects.PaymentMethod `json:"payment_method"`
	AmountCents   int64                      `json:"amount_cents"`
	RefMonth      valueobjects.RefMonth      `json:"ref_month"`
	CategoryID    uuid.UUID                  `json:"category_id"`
	SubcategoryID uuid.UUID                  `json:"subcategory_id"`
}

type TransactionUpdated struct {
	EventID           uuid.UUID                  `json:"event_id"`
	AggregateID       uuid.UUID                  `json:"aggregate_id"`
	UserID            uuid.UUID                  `json:"user_id"`
	OccurredAt        time.Time                  `json:"occurred_at"`
	Direction         valueobjects.Direction     `json:"direction"`
	PaymentMethod     valueobjects.PaymentMethod `json:"payment_method"`
	AmountCents       int64                      `json:"amount_cents"`
	RefMonth          valueobjects.RefMonth      `json:"ref_month"`
	RefMonthsAffected []valueobjects.RefMonth    `json:"ref_months_affected"`
}

type TransactionDeleted struct {
	EventID           uuid.UUID               `json:"event_id"`
	AggregateID       uuid.UUID               `json:"aggregate_id"`
	UserID            uuid.UUID               `json:"user_id"`
	OccurredAt        time.Time               `json:"occurred_at"`
	RefMonth          valueobjects.RefMonth   `json:"ref_month"`
	RefMonthsAffected []valueobjects.RefMonth `json:"ref_months_affected"`
}

type CardPurchaseInstallment struct {
	ItemID      uuid.UUID             `json:"item_id"`
	RefMonth    valueobjects.RefMonth `json:"ref_month"`
	AmountCents int64                 `json:"amount_cents"`
	Index       int                   `json:"index"`
}

type CardPurchaseCreated struct {
	EventID           uuid.UUID                 `json:"event_id"`
	AggregateID       uuid.UUID                 `json:"aggregate_id"`
	UserID            uuid.UUID                 `json:"user_id"`
	OccurredAt        time.Time                 `json:"occurred_at"`
	CardID            uuid.UUID                 `json:"card_id"`
	SubcategoryID     uuid.UUID                 `json:"subcategory_id"`
	TotalAmountCents  int64                     `json:"total_amount_cents"`
	InstallmentsTotal int                       `json:"installments_total"`
	RefMonthsAffected []valueobjects.RefMonth   `json:"ref_months_affected"`
	Installments      []CardPurchaseInstallment `json:"installments"`
}

type CardPurchaseUpdated struct {
	EventID           uuid.UUID               `json:"event_id"`
	AggregateID       uuid.UUID               `json:"aggregate_id"`
	UserID            uuid.UUID               `json:"user_id"`
	OccurredAt        time.Time               `json:"occurred_at"`
	CardID            uuid.UUID               `json:"card_id"`
	TotalAmountCents  int64                   `json:"total_amount_cents"`
	InstallmentsTotal int                     `json:"installments_total"`
	RefMonthsAffected []valueobjects.RefMonth `json:"ref_months_affected"`
	InvoiceDeltas     map[string]int64        `json:"invoice_deltas"`
}

type CardPurchaseDeleted struct {
	EventID           uuid.UUID               `json:"event_id"`
	AggregateID       uuid.UUID               `json:"aggregate_id"`
	UserID            uuid.UUID               `json:"user_id"`
	OccurredAt        time.Time               `json:"occurred_at"`
	CardID            uuid.UUID               `json:"card_id"`
	RefMonthsAffected []valueobjects.RefMonth `json:"ref_months_affected"`
	InvoiceDeltas     map[string]int64        `json:"invoice_deltas"`
}
