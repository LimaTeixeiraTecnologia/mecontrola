package output

import "time"

type Card struct {
	ID         string
	UserID     string
	Name       string
	Nickname   string
	ClosingDay int
	DueDay     int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}
