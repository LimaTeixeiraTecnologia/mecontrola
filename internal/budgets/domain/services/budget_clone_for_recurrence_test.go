package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type BudgetClonerForRecurrenceSuite struct {
	suite.Suite
}

func TestBudgetClonerForRecurrenceSuite(t *testing.T) {
	suite.Run(t, new(BudgetClonerForRecurrenceSuite))
}

func compClone(s *BudgetClonerForRecurrenceSuite, raw string) valueobjects.Competence {
	c, err := valueobjects.NewCompetence(raw)
	s.Require().NoError(err)
	return c
}

func sourceActive(s *BudgetClonerForRecurrenceSuite, total int64) entities.Budget {
	id := uuid.New()
	now := time.Now().UTC()
	slugs := valueobjects.CanonicalOrder()
	bps := [5]int{5000, 2000, 1500, 1000, 500}
	allocs := make([]entities.Allocation, 0, 5)
	for i, bp := range bps {
		allocs = append(allocs, entities.NewAllocation(id, slugs[i], bp, 0))
	}
	return entities.HydrateBudget(id, uuid.New(), compClone(s, "2026-06"), total, entities.BudgetStateActive, &now, false, allocs, now, now)
}

func (s *BudgetClonerForRecurrenceSuite) TestClone() {
	now := time.Now().UTC()
	cloner := services.NewBudgetClonerForRecurrence(nil)

	s.Run("clonagem feliz preserva total, allocations e gera competência alvo", func() {
		src := sourceActive(s, 100000)
		target := compClone(s, "2026-07")
		userID := uuid.New()

		clone, err := cloner.Clone(src, target, userID, now)
		s.Require().NoError(err)
		s.Equal(int64(100000), clone.TotalCents())
		s.Equal(target, clone.Competence())
		s.Equal(userID, clone.UserID())
		s.True(clone.IsDraft())
		s.Len(clone.Allocations(), 5)
		var sum int64
		for _, a := range clone.Allocations() {
			sum += a.PlannedCents()
		}
		s.Equal(int64(100000), sum)
	})

	s.Run("fonte inválida retorna erro com ErrRecurrenceCloneInvalidSource", func() {
		id := uuid.New()
		nowLocal := time.Now().UTC()
		invalid := entities.HydrateBudget(id, uuid.New(), compClone(s, "2026-06"), 0, entities.BudgetStateActive, &nowLocal, false, nil, nowLocal, nowLocal)
		_, err := cloner.Clone(invalid, compClone(s, "2026-07"), uuid.New(), now)
		s.Require().Error(err)
		s.ErrorIs(err, services.ErrRecurrenceCloneInvalidSource)
		s.ErrorIs(err, services.ErrRecurrenceSourceNegativeTotal)
	})

	s.Run("Rebase reconstrói allocations para budget existente", func() {
		src := sourceActive(s, 50000)
		targetBudget := entities.NewBudget(uuid.New(), compClone(s, "2026-08"), 50000, now)
		allocs := cloner.Rebase(targetBudget, src)
		s.Len(allocs, 5)
		for _, a := range allocs {
			s.Equal(targetBudget.ID(), a.BudgetID())
		}
	})
}
