package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type CardInvoiceRepository interface {
	UpsertByMonth(ctx context.Context, userID, cardID uuid.UUID, refMonth valueobjects.RefMonth, closingAt, dueAt time.Time) (*entities.CardInvoice, error)
	ApplyDelta(ctx context.Context, invoiceID uuid.UUID, deltaCents int64, expectedVersion int64) error
	GetByMonth(ctx context.Context, userID, cardID uuid.UUID, refMonth valueobjects.RefMonth) (*entities.CardInvoice, []*entities.CardInvoiceItem, error)
	SumByMonth(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth) (outcomeCents int64, err error)
}
