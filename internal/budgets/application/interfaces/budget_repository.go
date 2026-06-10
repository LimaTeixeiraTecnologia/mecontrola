package interfaces

import (
	"context"
	"errors"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrBudgetNotFound = errors.New("budgets: orçamento não encontrado")

var ErrBudgetConflict = errors.New("budgets: conflito de orçamento")

type BudgetRepository interface {
	GetByUserCompetence(ctx context.Context, db database.DBTX, userID uuid.UUID, c valueobjects.Competence) (entities.Budget, error)
	CreateDraft(ctx context.Context, db database.DBTX, b entities.Budget) error
	Activate(ctx context.Context, db database.DBTX, b entities.Budget) error
	DeleteDraft(ctx context.Context, db database.DBTX, userID uuid.UUID, c valueobjects.Competence) error
	ListFutureNotActivated(ctx context.Context, db database.DBTX, userID uuid.UUID, from valueobjects.Competence, max int) ([]entities.Budget, error)
	ListAbandonedDrafts(ctx context.Context, db database.DBTX, before valueobjects.Competence, limit int) ([]entities.Budget, error)
	SignalAbandoned(ctx context.Context, db database.DBTX, budgetID uuid.UUID) error
	IsSignaledAbandoned(ctx context.Context, db database.DBTX, budgetID uuid.UUID) (bool, error)
}
