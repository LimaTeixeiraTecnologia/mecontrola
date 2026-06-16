package input

import "github.com/google/uuid"

type UpdateCardLimit struct {
	CardID          uuid.UUID
	UserID          uuid.UUID
	LimitCents      int64
	ExpectedVersion *int64
}
