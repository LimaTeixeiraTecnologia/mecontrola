package interfaces

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ActiveCardForScan struct {
	UserID     uuid.UUID
	CardID     uuid.UUID
	LimitCents int64
	SpentCents int64
}

type CardThresholdReader interface {
	ListActiveCardsForThresholdScan(ctx context.Context, refMonth valueobjects.Competence, limit int) ([]ActiveCardForScan, error)
}

type CardThresholdReaderFactory interface {
	CardThresholdReader(db database.DBTX) CardThresholdReader
}
