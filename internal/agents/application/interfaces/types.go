package interfaces

import (
	"time"

	"github.com/google/uuid"
)

type EntryRef struct {
	ID         uuid.UUID
	Kind       EntryKind
	Reconciled bool
}

type RawTransaction struct {
	Direction           string
	PaymentMethod       string
	AmountCents         int64
	Description         string
	CategoryID          uuid.UUID
	SubcategoryID       *uuid.UUID
	CardID              *uuid.UUID
	Installments        int
	OccurredAt          string
	OriginWamid         string
	OriginItemSeq       int
	OriginOperation     string
	CategorySource      string
	CategoryOutcome     string
	CategoryScore       float64
	CategoryConfidence  string
	CategoryQuality     string
	CategorySignalType  string
	CategoryMatchedTerm string
	CategoryMatchReason string
	CategoryVersion     int64
}

type RawUpdateTransaction struct {
	ID                  uuid.UUID
	Direction           string
	PaymentMethod       string
	AmountCents         int64
	Description         string
	CategoryID          uuid.UUID
	SubcategoryID       *uuid.UUID
	OccurredAt          string
	Version             int64
	CategorySource      string
	CategoryOutcome     string
	CategoryScore       float64
	CategoryConfidence  string
	CategoryQuality     string
	CategorySignalType  string
	CategoryMatchedTerm string
	CategoryMatchReason string
	CategoryVersion     int64
}

type MonthlyEntry struct {
	Kind        EntryKind
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
	ID              string
	UserID          string
	Nickname        string
	Bank            string
	ClosingDay      int
	DueDay          int
	BestPurchaseDay int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type BestPurchaseDay struct {
	ClosingDay      int
	BestPurchaseDay int
}

type CardUpdate struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	Nickname *string
	Bank     *string
	DueDay   *int
}

type CardInvoiceItem struct {
	ID               uuid.UUID
	InvoiceID        uuid.UUID
	RefMonth         string
	InstallmentIndex int
	AmountCents      int64
}

type CardInvoice struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	CardID          uuid.UUID
	RefMonth        string
	ClosingAt       time.Time
	DueAt           time.Time
	ItemsTotalCents int64
	Version         int64
	Items           []CardInvoiceItem
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Entry struct {
	Kind                    EntryKind
	ID                      string
	UserID                  string
	Direction               string
	PaymentMethod           string
	AmountCents             int64
	Description             string
	CategoryID              string
	SubcategoryID           *string
	CategoryNameSnapshot    string
	SubcategoryNameSnapshot string
	RefMonth                string
	OccurredAt              time.Time
	Version                 int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type Recurrence struct {
	ID                      uuid.UUID
	UserID                  uuid.UUID
	Direction               string
	PaymentMethod           string
	CardID                  *uuid.UUID
	AmountCents             int64
	Description             string
	CategoryID              uuid.UUID
	SubcategoryID           *uuid.UUID
	CategoryNameSnapshot    string
	SubcategoryNameSnapshot string
	Frequency               string
	DayOfMonth              int
	InstallmentsTotal       int
	StartedAt               time.Time
	EndedAt                 *time.Time
	Version                 int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type RawRecurrence struct {
	UserID        uuid.UUID
	Direction     string
	PaymentMethod string
	CardID        *uuid.UUID
	AmountCents   int64
	Description   string
	CategoryID    uuid.UUID
	SubcategoryID *uuid.UUID
	Frequency     string
	DayOfMonth    int
	StartedAt     string
}

type RawUpdateRecurrence struct {
	Direction     *string
	PaymentMethod *string
	AmountCents   *int64
	Description   *string
	CategoryID    *uuid.UUID
	SubcategoryID *uuid.UUID
	Frequency     *string
	DayOfMonth    *int
	EndedAt       *string
	Version       int64
}

type AllocationBP struct {
	RootSlug    string
	BasisPoints int
}

type AllocationCents struct {
	RootSlug     string
	BasisPoints  int
	PlannedCents int64
}

type Category struct {
	ID             uuid.UUID
	Slug           string
	Name           string
	Kind           string
	ParentID       *uuid.UUID
	AllocationType string
	Subcategories  []Category
}

type CategoryCandidate struct {
	CategoryID     uuid.UUID
	RootCategoryID uuid.UUID
	Path           string
	MatchedTerm    string
	SignalType     string
	Confidence     string
	MatchQuality   string
	Score          float64
	IsAmbiguous    bool
	MatchReason    string
}

type CategorySearchResult struct {
	Outcome    ClassifyOutcome
	Version    int64
	HasMore    bool
	Candidates []CategoryCandidate
}

type CategoryWriteRequest struct {
	RootCategoryID  uuid.UUID
	SubcategoryID   uuid.UUID
	Kind            CategoryKind
	ExpectedVersion int64
}

type CategoryWriteDecision struct {
	RootCategoryID   uuid.UUID
	SubcategoryID    uuid.UUID
	Kind             CategoryKind
	Path             string
	EditorialVersion int64
	Deprecated       bool
}
