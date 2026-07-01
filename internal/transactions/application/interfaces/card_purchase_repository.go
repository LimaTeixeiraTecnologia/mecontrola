package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type CardPurchaseRepository interface {
	Create(ctx context.Context, p *entities.CardPurchase) (uuid.UUID, bool, error)
	UpdateWithVersion(ctx context.Context, p *entities.CardPurchase, expectedVersion int64) error
	SoftDelete(ctx context.Context, id, userID uuid.UUID, expectedVersion int64, now time.Time) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*entities.CardPurchase, error)
	ListByCardAndMonth(ctx context.Context, userID, cardID uuid.UUID, refMonth *valueobjects.RefMonth, cursor Cursor, limit int) ([]*entities.CardPurchase, Cursor, error)
	ReplaceItems(ctx context.Context, purchaseID uuid.UUID, items []*entities.CardInvoiceItem) error
	ExistsActiveByCardAndUser(ctx context.Context, cardID, userID uuid.UUID) (bool, error)
}
