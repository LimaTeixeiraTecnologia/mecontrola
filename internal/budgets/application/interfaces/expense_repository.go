package interfaces

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrExpenseNotFound = errors.New("budgets: despesa não encontrada")

var ErrExpenseConflict = errors.New("budgets: conflito de versão na despesa")

var ErrExpenseTombstoneConflict = errors.New("budgets: identidade canônica bloqueada por tombstone")

type ExpenseRepository interface {
	GetByIdentity(ctx context.Context, k entities.ExpenseIdentity) (entities.Expense, entities.ExpenseTombstone, error)
	Insert(ctx context.Context, e entities.Expense) error
	Update(ctx context.Context, e entities.Expense, expectedVersion int64) error
	SoftDelete(ctx context.Context, e entities.Expense, expectedVersion int64) (tombstoneVersion int64, err error)
	SumByRoot(ctx context.Context, userID uuid.UUID, c valueobjects.Competence) (map[valueobjects.RootSlug]int64, error)
	PurgeDeleted(ctx context.Context, olderThan string, limit int) (int64, error)
}
