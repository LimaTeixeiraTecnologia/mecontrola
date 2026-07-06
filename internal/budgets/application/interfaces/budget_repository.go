package interfaces

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrBudgetNotFound = errors.New("budgets: orçamento não encontrado")

var ErrBudgetConflict = errors.New("budgets: conflito de orçamento")

var ErrBudgetAlreadyActive = entities.ErrBudgetAlreadyActive

type BudgetRepository interface {
	GetByUserCompetence(ctx context.Context, userID uuid.UUID, c valueobjects.Competence) (entities.Budget, error)
	CreateDraft(ctx context.Context, b entities.Budget) error
	Activate(ctx context.Context, b entities.Budget) error
	DeleteDraft(ctx context.Context, userID uuid.UUID, c valueobjects.Competence) error
	ListFutureNotActivated(ctx context.Context, userID uuid.UUID, from valueobjects.Competence, max int) ([]entities.Budget, error)
	ListAbandonedDrafts(ctx context.Context, before valueobjects.Competence, limit int) ([]entities.Budget, error)
	SignalAbandoned(ctx context.Context, budgetID uuid.UUID) error
	IsSignaledAbandoned(ctx context.Context, budgetID uuid.UUID) (bool, error)
}
