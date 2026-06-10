package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
)

type CreateBudgetSuite struct {
	suite.Suite
	ctx     context.Context
	repo    *mockInterfaces.BudgetRepository
	uow     *uowMocks.UnitOfWorkBudget
	useCase *usecases.CreateBudget
}

func TestCreateBudgetSuite(t *testing.T) {
	suite.Run(t, new(CreateBudgetSuite))
}

func (s *CreateBudgetSuite) SetupTest() {
	s.ctx = context.Background()
	s.repo = mockInterfaces.NewBudgetRepository(s.T())
	s.uow = uowMocks.NewUnitOfWorkBudget(s.T())
	s.useCase = usecases.NewCreateBudget(s.repo, s.uow, noop.NewProvider())
}

func (s *CreateBudgetSuite) TestExecute_ValidInput_WithAllocations() {
	userID := uuid.New().String()

	s.repo.EXPECT().
		CreateDraft(s.ctx, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.CreateBudgetInput{
		UserID:     userID,
		Competence: "2026-06",
		TotalCents: 100000,
		Allocations: []input.AllocationInput{
			{RootSlug: "expense.custo_fixo", BasisPoints: 5000},
			{RootSlug: "expense.conhecimento", BasisPoints: 5000},
		},
	})

	s.NoError(err)
	s.Equal(userID, result.UserID)
	s.Equal("2026-06", result.Competence)
	s.Equal(int64(100000), result.TotalCents)
	s.Equal("draft", result.State)
	s.Len(result.Allocations, 2)
}

func (s *CreateBudgetSuite) TestExecute_ValidInput_NoAllocations() {
	userID := uuid.New().String()

	s.repo.EXPECT().
		CreateDraft(s.ctx, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.CreateBudgetInput{
		UserID:     userID,
		Competence: "2026-06",
		TotalCents: 50000,
	})

	s.NoError(err)
	s.Equal("draft", result.State)
	s.Empty(result.Allocations)
}

func (s *CreateBudgetSuite) TestExecute_InvalidUserID() {
	_, err := s.useCase.Execute(s.ctx, input.CreateBudgetInput{
		UserID:     "not-a-uuid",
		Competence: "2026-06",
		TotalCents: 100000,
	})

	s.ErrorIs(err, usecases.ErrBudgetInvalidUserID)
}

func (s *CreateBudgetSuite) TestExecute_InvalidCompetence() {
	_, err := s.useCase.Execute(s.ctx, input.CreateBudgetInput{
		UserID:     uuid.New().String(),
		Competence: "2026-13",
		TotalCents: 100000,
	})

	s.ErrorIs(err, usecases.ErrBudgetInvalidCompetence)
}

func (s *CreateBudgetSuite) TestExecute_InvalidAllocationRootSlug() {
	_, err := s.useCase.Execute(s.ctx, input.CreateBudgetInput{
		UserID:     uuid.New().String(),
		Competence: "2026-06",
		TotalCents: 100000,
		Allocations: []input.AllocationInput{
			{RootSlug: "invalid.slug", BasisPoints: 5000},
		},
	})

	s.ErrorIs(err, usecases.ErrBudgetInvalidAllocationRootSlug)
}

func (s *CreateBudgetSuite) TestExecute_BasisPointsNegative() {
	_, err := s.useCase.Execute(s.ctx, input.CreateBudgetInput{
		UserID:     uuid.New().String(),
		Competence: "2026-06",
		TotalCents: 100000,
		Allocations: []input.AllocationInput{
			{RootSlug: "expense.custo_fixo", BasisPoints: -1},
		},
	})

	s.ErrorIs(err, usecases.ErrBudgetAllocationBasisPointsInvalid)
}

func (s *CreateBudgetSuite) TestExecute_BasisPointsAbove10000() {
	_, err := s.useCase.Execute(s.ctx, input.CreateBudgetInput{
		UserID:     uuid.New().String(),
		Competence: "2026-06",
		TotalCents: 100000,
		Allocations: []input.AllocationInput{
			{RootSlug: "expense.custo_fixo", BasisPoints: 10001},
		},
	})

	s.ErrorIs(err, usecases.ErrBudgetAllocationBasisPointsInvalid)
}

func (s *CreateBudgetSuite) TestExecute_AllocationSumExceeds10000() {
	_, err := s.useCase.Execute(s.ctx, input.CreateBudgetInput{
		UserID:     uuid.New().String(),
		Competence: "2026-06",
		TotalCents: 100000,
		Allocations: []input.AllocationInput{
			{RootSlug: "expense.custo_fixo", BasisPoints: 6000},
			{RootSlug: "expense.conhecimento", BasisPoints: 6000},
		},
	})

	s.ErrorIs(err, usecases.ErrBudgetAllocationSumExceeds10000)
}

func (s *CreateBudgetSuite) TestExecute_Conflict() {
	userID := uuid.New().String()

	s.repo.EXPECT().
		CreateDraft(s.ctx, mock.Anything, mock.Anything).
		Return(interfaces.ErrBudgetConflict).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.CreateBudgetInput{
		UserID:     userID,
		Competence: "2026-06",
		TotalCents: 100000,
	})

	s.ErrorIs(err, interfaces.ErrBudgetConflict)
}

func (s *CreateBudgetSuite) TestExecute_RepositoryError() {
	userID := uuid.New().String()

	s.repo.EXPECT().
		CreateDraft(s.ctx, mock.Anything, mock.Anything).
		Return(errors.New("db error")).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.CreateBudgetInput{
		UserID:     userID,
		Competence: "2026-06",
		TotalCents: 100000,
	})

	s.Error(err)
}
