package interfaces

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var ErrTransactionNotFound = errors.New("transactions: lançamento não encontrado")
var ErrTransactionVersionConflict = errors.New("transactions: conflito de versão")

type TransactionRepository interface {
	Create(ctx context.Context, tx *entities.Transaction) error
	UpdateWithVersion(ctx context.Context, tx *entities.Transaction, expectedVersion int64) error
	SoftDelete(ctx context.Context, id uuid.UUID, userID uuid.UUID, expectedVersion int64, now time.Time) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*entities.Transaction, error)
	ListByMonth(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth, cursor Cursor, limit int) ([]*entities.Transaction, Cursor, error)
	SumByMonth(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth) (incomeCents, outcomeCents int64, err error)
}
