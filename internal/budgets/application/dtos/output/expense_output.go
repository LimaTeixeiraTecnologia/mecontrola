package output

import "time"

type ExpenseOutput struct {
	ID                    string
	UserID                string
	Source                string
	ExternalTransactionID string
	SubcategoryID         string
	RootSlug              string
	Competence            string
	AmountCents           int64
	OccurredAt            time.Time
	Version               int64
	TombstoneVersion      *int64
	DeletedAt             *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
