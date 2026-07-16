package interfaces

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var ErrTransactionNotFound = errors.New("transactions: lançamento não encontrado")
var ErrTransactionVersionConflict = errors.New("transactions: conflito de versão")

type TransactionRepository interface {
	Create(ctx context.Context, tx *entities.Transaction) (uuid.UUID, bool, error)
	UpdateWithVersion(ctx context.Context, tx *entities.Transaction, expectedVersion int64) error
	SoftDelete(ctx context.Context, id uuid.UUID, userID uuid.UUID, expectedVersion int64, now time.Time) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*entities.Transaction, error)
	GetItemsByTransactionID(ctx context.Context, txID uuid.UUID) ([]*entities.CardInvoiceItem, error)
	ReplaceItems(ctx context.Context, txID uuid.UUID, items []*entities.CardInvoiceItem) error
	ExistsActiveCreditByCard(ctx context.Context, cardID, userID uuid.UUID) (bool, error)
	ListByMonth(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth, cursor Cursor, limit int) ([]*entities.Transaction, Cursor, error)
	SearchByDescription(ctx context.Context, userID uuid.UUID, q valueobjects.SearchQuery, refMonth option.Option[valueobjects.RefMonth], limit int) ([]*entities.Transaction, error)
	SearchEditCandidates(ctx context.Context, userID uuid.UUID, amountCents int64, term string, refMonth valueobjects.RefMonth, limit int) ([]*entities.Transaction, error)
	SumByMonthExcludingCredit(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth) (incomeCents, outcomeCents int64, err error)
}
