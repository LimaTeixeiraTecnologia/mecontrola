package output

import "time"

type Card struct {
	ID              string     `json:"id"`
	UserID          string     `json:"user_id"`
	Nickname        string     `json:"nickname"`
	Bank            string     `json:"bank"`
	ClosingDay      int        `json:"closing_day"`
	DueDay          int        `json:"due_day"`
	BestPurchaseDay int        `json:"best_purchase_day"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}
