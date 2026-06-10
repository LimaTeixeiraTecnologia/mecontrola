package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type CreateOrAutoDraftForExpense struct {
	budgets interfaces.BudgetRepository
}

func NewCreateOrAutoDraftForExpense(budgets interfaces.BudgetRepository) *CreateOrAutoDraftForExpense {
	return &CreateOrAutoDraftForExpense{budgets: budgets}
}

func (uc *CreateOrAutoDraftForExpense) EnsureExists(ctx context.Context, tx database.DBTX, userID uuid.UUID, competence valueobjects.Competence, now time.Time) error {
	_, err := uc.budgets.GetByUserCompetence(ctx, tx, userID, competence)
	if err == nil {
		return nil
	}

	if !errors.Is(err, interfaces.ErrBudgetNotFound) {
		return fmt.Errorf("budgets.usecase.create_or_auto_draft: verificar orçamento: %w", err)
	}

	draft := entities.NewAutoDraftBudget(userID, competence, now)
	if createErr := uc.budgets.CreateDraft(ctx, tx, draft); createErr != nil {
		if errors.Is(createErr, interfaces.ErrBudgetConflict) {
			return nil
		}
		return fmt.Errorf("budgets.usecase.create_or_auto_draft: criar rascunho automático: %w", createErr)
	}

	return nil
}
