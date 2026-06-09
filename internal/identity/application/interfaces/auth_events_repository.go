package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type AuthEventsRepository interface {
	Insert(ctx context.Context, event entities.AuthEvent) error
	AnonymizeByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteOlderThan(ctx context.Context, cutoff time.Time, batchSize int) (int64, error)
}
