package interfaces

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type WhatsAppHistoryEntry struct {
	ID         string
	UserID     string
	Number     string
	Active     bool
	LinkedAt   time.Time
	UnlinkedAt time.Time
	Reason     string
}

type UserRepository interface {
	UpsertByWhatsAppNumber(ctx context.Context, u entities.User, now time.Time) (entities.User, error)
	FindByID(ctx context.Context, id string) (entities.User, error)
	FindByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (entities.User, error)
	FindByWhatsAppNumberIncludingDeleted(ctx context.Context, number valueobjects.WhatsAppNumber) (entities.User, error)
	Reanimate(ctx context.Context, u entities.User, now time.Time) (entities.User, error)
	MarkDeleted(ctx context.Context, id string, now time.Time) error
	AppendWhatsAppHistory(ctx context.Context, userID string, entry WhatsAppHistoryEntry) error
}
