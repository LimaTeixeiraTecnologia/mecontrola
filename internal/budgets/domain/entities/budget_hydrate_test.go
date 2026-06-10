package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type BudgetHydrateSuite struct {
	suite.Suite
	now        time.Time
	userID     uuid.UUID
	competence valueobjects.Competence
}

func TestBudgetHydrateSuite(t *testing.T) {
	suite.Run(t, new(BudgetHydrateSuite))
}

func (s *BudgetHydrateSuite) SetupTest() {
	s.now = time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.userID = uuid.New()
	c, _ := valueobjects.NewCompetence("2025-06")
	s.competence = c
}

func (s *BudgetHydrateSuite) TestHydrateBudget() {
	id := uuid.New()
	activatedAt := s.now.Add(time.Hour)
	allocs := []entities.Allocation{
		entities.NewAllocation(id, valueobjects.RootSlugCustoFixo, 10000, 1000),
	}
	b := entities.HydrateBudget(id, s.userID, s.competence, 1000, entities.BudgetStateActive, &activatedAt, false, allocs, s.now, s.now)

	s.Equal(id, b.ID())
	s.Equal(s.userID, b.UserID())
	s.Equal(s.competence, b.Competence())
	s.Equal(int64(1000), b.TotalCents())
	s.Equal(entities.BudgetStateActive, b.State())
	s.True(b.IsActive())
	s.False(b.IsDraft())
	s.NotNil(b.ActivatedAt())
	s.False(b.AutoDraft())
	s.Len(b.Allocations(), 1)
	s.Equal(s.now, b.CreatedAt())
	s.Equal(s.now, b.UpdatedAt())
}

func (s *BudgetHydrateSuite) TestNewAutoDraftBudget() {
	b := entities.NewAutoDraftBudget(s.userID, s.competence, s.now)
	s.True(b.AutoDraft())
	s.True(b.IsDraft())
	s.Equal(int64(0), b.TotalCents())
	s.Nil(b.ActivatedAt())
}

func (s *BudgetHydrateSuite) TestActivateAlreadyActive() {
	id := uuid.New()
	activatedAt := s.now
	b := entities.HydrateBudget(id, s.userID, s.competence, 1000, entities.BudgetStateActive, &activatedAt, false, nil, s.now, s.now)
	err := b.Activate(s.now)
	s.ErrorIs(err, entities.ErrBudgetAlreadyActive)
}
