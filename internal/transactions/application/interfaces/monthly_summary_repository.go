package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type MonthlyEntry struct {
	Kind        string
	ID          string
	UserID      uuid.UUID
	RefMonth    string
	AmountCents int64
	Direction   string
	Description string
	CreatedAt   time.Time
}

type MonthlySummaryRepository interface {
	Upsert(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth, incomeCents, outcomeCents int64, updatedAt time.Time) error
	Get(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth) (*entities.MonthlySummary, error)
	ListActiveSince(ctx context.Context, since time.Time, cursor Cursor, batchSize int) ([]MonthlySummaryKey, Cursor, error)
	ListEntries(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth, cursor Cursor, limit int) ([]MonthlyEntry, Cursor, error)
}
