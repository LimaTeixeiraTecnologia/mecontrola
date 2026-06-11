package mappers

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

func (Mapper) Expense(e entities.Expense) output.ExpenseOutput {
	return output.ExpenseOutput{
		ID:                    e.ID().String(),
		UserID:                e.UserID().String(),
		Source:                e.Source().String(),
		ExternalTransactionID: e.ExternalTransactionID().String(),
		SubcategoryID:         e.SubcategoryID().String(),
		RootSlug:              e.RootSlug().String(),
		Competence:            e.Competence().String(),
		AmountCents:           e.AmountCents(),
		OccurredAt:            e.OccurredAt(),
		Version:               e.Version(),
		TombstoneVersion:      e.TombstoneVersion(),
		DeletedAt:             e.DeletedAt(),
		CreatedAt:             e.CreatedAt(),
		UpdatedAt:             e.UpdatedAt(),
	}
}
