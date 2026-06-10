package output

type AllocationSummary struct {
	RootSlug        string   `json:"root_slug"`
	PlannedCents    *int64   `json:"planned_cents"`
	SpentCents      int64    `json:"spent_cents"`
	PercentageSpent *float64 `json:"percentage_spent"`
}

type MonthlySummaryOutput struct {
	UserID            string              `json:"user_id"`
	Competence        string              `json:"competence"`
	TotalCents        *int64              `json:"total_cents"`
	AutoDraft         bool                `json:"auto_draft"`
	State             string              `json:"state"`
	Allocations       []AllocationSummary `json:"allocations"`
	TotalSpentCents   int64               `json:"total_spent_cents"`
	TotalPlannedCents *int64              `json:"total_planned_cents"`
	PercentageTotal   *float64            `json:"percentage_total"`
}
