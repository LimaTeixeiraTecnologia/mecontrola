package interfaces

import (
	"context"
	"errors"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrExpenseNotFound = errors.New("budgets: despesa não encontrada")

var ErrExpenseConflict = errors.New("budgets: conflito de versão na despesa")

var ErrExpenseTombstoneConflict = errors.New("budgets: identidade canônica bloqueada por tombstone")

type ExpenseRepository interface {
	GetByIdentity(ctx context.Context, db database.DBTX, k entities.ExpenseIdentity) (entities.Expense, entities.ExpenseTombstone, error)
	Insert(ctx context.Context, db database.DBTX, e entities.Expense) error
	Update(ctx context.Context, db database.DBTX, e entities.Expense, expectedVersion int64) error
	SoftDelete(ctx context.Context, db database.DBTX, e entities.Expense, expectedVersion int64) (tombstoneVersion int64, err error)
	SumByRoot(ctx context.Context, db database.DBTX, userID uuid.UUID, c valueobjects.Competence) (map[valueobjects.RootSlug]int64, error)
	PurgeDeleted(ctx context.Context, db database.DBTX, olderThan string, limit int) (int64, error)
}
