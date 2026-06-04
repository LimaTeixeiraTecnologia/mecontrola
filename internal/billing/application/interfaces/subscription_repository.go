package interfaces

import (
	"context"
	"errors"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

// ErrSubscriptionNotFound é o sentinel retornado por SubscriptionRepository quando
// nenhuma subscription ativa satisfaz os critérios da query.
// Repositórios concretos devem retornar exatamente este sentinel (sem wrapping extra)
// para que os use cases possam usar errors.Is.
var ErrSubscriptionNotFound = errors.New("billing: subscription não encontrada")

// SubscriptionRepository é o port de persistência do agregado Subscription.
// FindActiveByUserIDForUpdate adquire SELECT ... FOR UPDATE para garantir serialização
// no processor de eventos (ADR-012).
type SubscriptionRepository interface {
	Upsert(ctx context.Context, sub *entities.Subscription) error
	FindActiveByUserID(ctx context.Context, userID identityentities.UserID) (*entities.Subscription, error)
	// FindActiveByUserIDForUpdate adquire lock pessimista (SELECT ... FOR UPDATE).
	// Deve ser chamado dentro de uma UnitOfWork ativa.
	FindActiveByUserIDForUpdate(ctx context.Context, userID identityentities.UserID) (*entities.Subscription, error)
	FindByExternalID(ctx context.Context, provider string, externalID valueobjects.ExternalSubscriptionID) (*entities.Subscription, error)
	// ListByStatusInBatch retorna subscriptions por cursor composto (cursorCreatedAt, cursorID)
	// para paginação estável sem offset.
	ListByStatusInBatch(ctx context.Context, statuses []valueobjects.SubscriptionStatus, cursorCreatedAt time.Time, cursorID entities.SubscriptionID, limit int) ([]*entities.Subscription, error)
}
