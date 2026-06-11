package services

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrRecurrenceCloneInvalidSource = errors.New("budgets.recurrence.clone: fonte inválida para clonagem")

type BudgetClonerForRecurrence struct {
	validator *RecurrenceSourceValidator
}

func NewBudgetClonerForRecurrence(validator *RecurrenceSourceValidator) *BudgetClonerForRecurrence {
	if validator == nil {
		validator = NewRecurrenceSourceValidator()
	}
	return &BudgetClonerForRecurrence{validator: validator}
}

func (c *BudgetClonerForRecurrence) Clone(source entities.Budget, target valueobjects.Competence, userID uuid.UUID, now time.Time) (entities.Budget, error) {
	if err := c.validator.Validate(source); err != nil {
		return entities.Budget{}, errors.Join(ErrRecurrenceCloneInvalidSource, err)
	}

	newBudget := entities.NewBudget(userID, target, source.TotalCents(), now)
	newBudget.SetAllocations(c.rebuildAllocations(newBudget.ID(), source))
	return newBudget, nil
}

func (c *BudgetClonerForRecurrence) Rebase(target entities.Budget, source entities.Budget) []entities.Allocation {
	return c.rebuildAllocations(target.ID(), source)
}

func (c *BudgetClonerForRecurrence) rebuildAllocations(budgetID uuid.UUID, source entities.Budget) []entities.Allocation {
	sourceAllocs := source.Allocations()
	inputs := make([]AllocationInput, 0, len(sourceAllocs))
	for _, a := range sourceAllocs {
		inputs = append(inputs, AllocationInput{
			RootSlug:    a.RootSlug(),
			BasisPoints: a.BasisPoints(),
		})
	}
	distributed := Distribute(source.TotalCents(), inputs)
	allocs := make([]entities.Allocation, 0, len(distributed))
	for _, r := range distributed {
		allocs = append(allocs, entities.NewAllocation(budgetID, r.RootSlug, r.BasisPoints, r.PlannedCents))
	}
	return allocs
}
