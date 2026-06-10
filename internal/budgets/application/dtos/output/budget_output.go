package output

import "time"

type AllocationOutput struct {
	RootSlug     string
	BasisPoints  int
	PlannedCents int64
}

type BudgetOutput struct {
	ID          string
	UserID      string
	Competence  string
	TotalCents  int64
	State       string
	AutoDraft   bool
	ActivatedAt *time.Time
	Allocations []AllocationOutput
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
