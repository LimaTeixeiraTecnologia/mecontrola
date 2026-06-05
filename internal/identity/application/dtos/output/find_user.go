package output

import "time"

type FindUser struct {
	ID             string
	WhatsAppNumber string
	Email          string
	DisplayName    string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
