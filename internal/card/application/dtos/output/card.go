package output

import "time"

type Card struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	Nickname   string     `json:"nickname"`
	ClosingDay int        `json:"closing_day"`
	DueDay     int        `json:"due_day"`
	LimitCents int64      `json:"limit_cents"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}
