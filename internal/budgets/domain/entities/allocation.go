package entities

import (
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type Allocation struct {
	budgetID     uuid.UUID
	rootSlug     valueobjects.RootSlug
	basisPoints  int
	plannedCents int64
}

func NewAllocation(budgetID uuid.UUID, rootSlug valueobjects.RootSlug, basisPoints int, plannedCents int64) Allocation {
	return Allocation{
		budgetID:     budgetID,
		rootSlug:     rootSlug,
		basisPoints:  basisPoints,
		plannedCents: plannedCents,
	}
}

func (a Allocation) BudgetID() uuid.UUID             { return a.budgetID }
func (a Allocation) RootSlug() valueobjects.RootSlug { return a.rootSlug }
func (a Allocation) BasisPoints() int                { return a.basisPoints }
func (a Allocation) PlannedCents() int64             { return a.plannedCents }

func (a *Allocation) SetPlannedCents(v int64) {
	a.plannedCents = v
}
