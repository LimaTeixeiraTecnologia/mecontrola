//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type BudgetRepositorySuite struct {
	suite.Suite
}

func TestBudgetRepositorySuite(t *testing.T) {
	suite.Run(t, new(BudgetRepositorySuite))
}

func (s *BudgetRepositorySuite) TestCreateAndGetDraft() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newBudgetRepo(testO11y(), mgr.DBTX(ctx))

	userID := uuid.New()
	competence := mustCompetence(s.T(), "2025-01")
	budget := newTestBudget(userID, competence)

	s.Require().NoError(repo.CreateDraft(ctx, budget))

	found, err := repo.GetByUserCompetence(ctx, userID, competence)
	s.Require().NoError(err)
	s.Assert().Equal(budget.ID(), found.ID())
	s.Assert().Equal(budget.TotalCents(), found.TotalCents())
	s.Assert().Equal(entities.BudgetStateDraft, found.State())
}

func (s *BudgetRepositorySuite) TestGetByUserCompetenceNotFound() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newBudgetRepo(testO11y(), mgr.DBTX(ctx))

	_, err := repo.GetByUserCompetence(ctx, uuid.New(), mustCompetence(s.T(), "2025-06"))
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrBudgetNotFound))
}

func (s *BudgetRepositorySuite) TestActivateBudget() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newBudgetRepo(testO11y(), mgr.DBTX(ctx))

	userID := uuid.New()
	competence := mustCompetence(s.T(), "2025-02")
	budget := newTestBudget(userID, competence)

	alloc := entities.NewAllocation(budget.ID(), mustRootSlug(s.T(), "expense.custo_fixo"), 10000, 500000)
	budget.SetAllocations([]entities.Allocation{alloc})

	s.Require().NoError(repo.CreateDraft(ctx, budget))

	s.Require().NoError(budget.Activate(time.Now().UTC()))
	s.Require().NoError(repo.Activate(ctx, budget))

	found, err := repo.GetByUserCompetence(ctx, userID, competence)
	s.Require().NoError(err)
	s.Assert().Equal(entities.BudgetStateActive, found.State())
	s.Assert().NotNil(found.ActivatedAt())
	s.Require().Len(found.Allocations(), 1)
	s.Assert().Equal(10000, found.Allocations()[0].BasisPoints())
}

func (s *BudgetRepositorySuite) TestDeleteDraft() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newBudgetRepo(testO11y(), mgr.DBTX(ctx))

	userID := uuid.New()
	competence := mustCompetence(s.T(), "2025-03")
	budget := newTestBudget(userID, competence)

	s.Require().NoError(repo.CreateDraft(ctx, budget))
	s.Require().NoError(repo.DeleteDraft(ctx, userID, competence))

	_, err := repo.GetByUserCompetence(ctx, userID, competence)
	s.Assert().True(errors.Is(err, interfaces.ErrBudgetNotFound))
}

func (s *BudgetRepositorySuite) TestDeleteDraftNotFound() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newBudgetRepo(testO11y(), mgr.DBTX(ctx))

	err := repo.DeleteDraft(ctx, uuid.New(), mustCompetence(s.T(), "2025-04"))
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrBudgetNotFound))
}

func (s *BudgetRepositorySuite) TestListFutureNotActivated() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newBudgetRepo(testO11y(), mgr.DBTX(ctx))

	userID := uuid.New()
	for _, comp := range []string{"2025-05", "2025-06", "2025-07"} {
		b := newTestBudget(userID, mustCompetence(s.T(), comp))
		s.Require().NoError(repo.CreateDraft(ctx, b))
	}

	from := mustCompetence(s.T(), "2025-05")
	results, err := repo.ListFutureNotActivated(ctx, userID, from, 10)
	s.Require().NoError(err)
	s.Assert().Len(results, 2)
	s.Assert().Equal("2025-06", results[0].Competence().String())
	s.Assert().Equal("2025-07", results[1].Competence().String())
}
