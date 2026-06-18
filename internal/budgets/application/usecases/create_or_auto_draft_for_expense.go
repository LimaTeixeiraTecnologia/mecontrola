package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type CreateOrAutoDraftForExpense struct {
	factory interfaces.RepositoryFactory
}

func NewCreateOrAutoDraftForExpense(factory interfaces.RepositoryFactory) *CreateOrAutoDraftForExpense {
	return &CreateOrAutoDraftForExpense{factory: factory}
}

func (uc *CreateOrAutoDraftForExpense) EnsureExists(ctx context.Context, tx database.DBTX, userID uuid.UUID, competence valueobjects.Competence, now time.Time) error {
	budgets := uc.factory.BudgetRepository(tx)
	_, err := budgets.GetByUserCompetence(ctx, userID, competence)
	if err == nil {
		return nil
	}

	if !errors.Is(err, interfaces.ErrBudgetNotFound) {
		return fmt.Errorf("budgets.usecase.create_or_auto_draft: verificar orçamento: %w", err)
	}

	draft := entities.NewAutoDraftBudget(userID, competence, now)
	if createErr := budgets.CreateDraft(ctx, draft); createErr != nil {
		if errors.Is(createErr, interfaces.ErrBudgetConflict) {
			return nil
		}
		return fmt.Errorf("budgets.usecase.create_or_auto_draft: criar rascunho automático: %w", createErr)
	}

	return nil
}
