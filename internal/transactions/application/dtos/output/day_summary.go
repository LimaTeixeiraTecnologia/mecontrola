package output

type DaySummary struct {
	Day          string         `json:"day"`
	IncomeCents  int64          `json:"income_cents"`
	OutcomeCents int64          `json:"outcome_cents"`
	TotalCents   int64          `json:"total_cents"`
	Entries      []MonthlyEntry `json:"entries"`
}
