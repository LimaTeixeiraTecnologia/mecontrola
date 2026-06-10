package entities_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type AllocationSuite struct {
	suite.Suite
}

func TestAllocationSuite(t *testing.T) {
	suite.Run(t, new(AllocationSuite))
}

func (s *AllocationSuite) TestNewAllocation() {
	budgetID := uuid.New()
	a := entities.NewAllocation(budgetID, valueobjects.RootSlugConhecimento, 2500, 250)
	s.Equal(budgetID, a.BudgetID())
	s.Equal(valueobjects.RootSlugConhecimento, a.RootSlug())
	s.Equal(2500, a.BasisPoints())
	s.Equal(int64(250), a.PlannedCents())
}

func (s *AllocationSuite) TestSetPlannedCents() {
	a := entities.NewAllocation(uuid.New(), valueobjects.RootSlugMetas, 1000, 0)
	a.SetPlannedCents(999)
	s.Equal(int64(999), a.PlannedCents())
}
