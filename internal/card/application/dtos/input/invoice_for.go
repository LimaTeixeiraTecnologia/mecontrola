package input

import (
	"time"

	"github.com/google/uuid"
)

type InvoiceFor struct {
	CardID   uuid.UUID
	UserID   uuid.UUID
	Purchase time.Time
}
