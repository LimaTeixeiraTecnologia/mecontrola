package interfaces

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
)

type CardRepository interface {
	Insert(ctx context.Context, c entities.Card) error
	GetByIDForUser(ctx context.Context, cardID, userID string) (entities.Card, error)
	ListByUser(ctx context.Context, userID, cursor string, limit int) ([]entities.Card, string, error)
	UpdateByIDForUser(ctx context.Context, c entities.Card) (entities.Card, error)
	SoftDeleteByIDForUser(ctx context.Context, cardID, userID string, now time.Time) error
}

type RepositoryFactory interface {
	CardRepository(db database.DBTX) CardRepository
}
