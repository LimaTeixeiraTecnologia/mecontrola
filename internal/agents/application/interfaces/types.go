package interfaces

import (
	"time"

	"github.com/google/uuid"
)

type EntryRef struct {
	ID         uuid.UUID
	Kind       string
	Reconciled bool
}

type RawTransaction struct {
	Direction       string
	PaymentMethod   string
	AmountCents     int64
	Description     string
	CategoryID      uuid.UUID
	SubcategoryID   *uuid.UUID
	OccurredAt      string
	OriginWamid     string
	OriginItemSeq   int
	OriginOperation string
}

type RawCardPurchase struct {
	CardID            uuid.UUID
	TotalAmountCents  int64
	InstallmentsTotal int
	Description       string
	CategoryID        uuid.UUID
	SubcategoryID     *uuid.UUID
	PurchasedAt       string
	OriginWamid       string
	OriginItemSeq     int
	OriginOperation   string
}

type RawUpdateTransaction struct {
	ID            uuid.UUID
	Direction     string
	PaymentMethod string
	AmountCents   int64
	Description   string
	CategoryID    uuid.UUID
	SubcategoryID *uuid.UUID
	OccurredAt    string
	Version       int64
}

type RawUpdateCardPurchase struct {
	ID                uuid.UUID
	TotalAmountCents  int64
	InstallmentsTotal int
	Description       string
	CategoryID        uuid.UUID
	SubcategoryID     *uuid.UUID
	PurchasedAt       string
	Version           int64
}

type MonthlyEntry struct {
	Kind        string
	ID          string
	RefMonth    string
	AmountCents int64
	Direction   string
	Description string
	CreatedAt   time.Time
}

type MonthlySummary struct {
	RefMonth     string
	IncomeCents  int64
	OutcomeCents int64
	TotalCents   int64
}

type AllocationDraft struct {
	RootSlug    string
	BasisPoints int
}

type DraftBudget struct {
	UserID      uuid.UUID
	Competence  string
	TotalCents  int64
	Allocations []AllocationDraft
}

type BudgetRef struct {
	ID         string
	Competence string
	State      string
}

type AllocationSummary struct {
	RootSlug        string
	PlannedCents    *int64
	SpentCents      int64
	PercentageSpent *float64
}

type BudgetSummary struct {
	Competence        string
	TotalCents        *int64
	State             string
	AutoDraft         bool
	Allocations       []AllocationSummary
	TotalSpentCents   int64
	TotalPlannedCents *int64
}

type Alert struct {
	ID           string
	Competence   string
	RootSlug     string
	Threshold    int
	State        string
	SpentCents   int64
	PlannedCents int64
}

type NewCard struct {
	UserID   uuid.UUID
	Nickname string
	Bank     string
	DueDay   int
}

type CardRef struct {
	ID       string
	Nickname string
}

type Card struct {
	ID         string
	Nickname   string
	Bank       string
	ClosingDay int
	DueDay     int
}

type CategoryCandidate struct {
	CategoryID     uuid.UUID
	RootCategoryID uuid.UUID
	Path           string
	MatchedTerm    string
	Score          float64
	IsAmbiguous    bool
}
