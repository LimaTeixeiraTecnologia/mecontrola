package output

import "time"

type AlertOutput struct {
	ID                     string    `json:"id"`
	UserID                 string    `json:"user_id"`
	Competence             string    `json:"competence"`
	RootSlug               string    `json:"root_slug"`
	Threshold              int       `json:"threshold"`
	State                  string    `json:"state"`
	TriggeredByCommittedAt time.Time `json:"triggered_by_committed_at"`
	SpentCents             int64     `json:"spent_cents"`
	PlannedCents           int64     `json:"planned_cents"`
	CreatedAt              time.Time `json:"created_at"`
}

type ListAlertsOutput struct {
	Alerts     []AlertOutput `json:"alerts"`
	NextCursor string        `json:"next_cursor,omitempty"`
}
