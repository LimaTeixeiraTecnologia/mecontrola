package interfaces

import (
	"context"

	"github.com/google/uuid"
)

type DayEntriesRepository interface {
	ListDayEntries(ctx context.Context, userID uuid.UUID, day string) ([]MonthlyEntry, error)
}
