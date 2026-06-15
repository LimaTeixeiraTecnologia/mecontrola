package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type UserIdentityRepository interface {
	TryFindActive(ctx context.Context, channel valueobjects.Channel, externalID valueobjects.ExternalID) (entities.UserIdentity, bool, error)
	FindByUserAndChannel(ctx context.Context, userID uuid.UUID, channel valueobjects.Channel) (entities.UserIdentity, bool, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]entities.UserIdentity, error)
	Insert(ctx context.Context, identity entities.UserIdentity) error
	Unlink(ctx context.Context, id uuid.UUID, now time.Time) error
}
