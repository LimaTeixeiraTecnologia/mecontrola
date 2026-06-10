package input

import "time"

type UpsertExpenseInput struct {
	UserID                string
	Source                string
	ExternalTransactionID string
	SubcategoryID         string
	Competence            string
	AmountCents           int64
	OccurredAt            time.Time
	ExpectedVersion       *int64
}
