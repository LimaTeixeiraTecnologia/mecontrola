package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type DeleteDraftBudgetSuite struct {
	suite.Suite
	ctx     context.Context
	repo    *mockInterfaces.BudgetRepository
	uow     *uowMocks.UnitOfWorkVoid
	useCase *usecases.DeleteDraftBudget
}

func TestDeleteDraftBudgetSuite(t *testing.T) {
	suite.Run(t, new(DeleteDraftBudgetSuite))
}

func (s *DeleteDraftBudgetSuite) SetupTest() {
	s.ctx = context.Background()
	s.repo = mockInterfaces.NewBudgetRepository(s.T())
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())
	s.useCase = usecases.NewDeleteDraftBudget(s.repo, s.uow, noop.NewProvider())
}

func (s *DeleteDraftBudgetSuite) TestExecute_InvalidUserID() {
	err := s.useCase.Execute(s.ctx, input.DeleteDraftInput{
		UserID:     "not-a-uuid",
		Competence: "2026-06",
	})

	s.ErrorIs(err, usecases.ErrBudgetInvalidUserID)
}

func (s *DeleteDraftBudgetSuite) TestExecute_InvalidCompetence() {
	err := s.useCase.Execute(s.ctx, input.DeleteDraftInput{
		UserID:     uuid.New().String(),
		Competence: "bad",
	})

	s.ErrorIs(err, usecases.ErrBudgetInvalidCompetence)
}

func (s *DeleteDraftBudgetSuite) TestExecute_BudgetNotFound() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	err := s.useCase.Execute(s.ctx, input.DeleteDraftInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.Error(err)
}

func (s *DeleteDraftBudgetSuite) TestExecute_ActiveBudgetRejected() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()

	activeBudget := entities.HydrateBudget(
		uuid.New(), userID, comp, 100000,
		entities.BudgetStateActive,
		&now,
		false,
		nil,
		now,
		now,
	)

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(activeBudget, nil).
		Once()

	err := s.useCase.Execute(s.ctx, input.DeleteDraftInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.ErrorIs(err, entities.ErrBudgetAlreadyActive)
}

func (s *DeleteDraftBudgetSuite) TestExecute_DraftDeletedSuccessfully() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()

	draft := entities.NewBudget(userID, comp, 100000, now)

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(draft, nil).
		Once()

	s.repo.EXPECT().
		DeleteDraft(s.ctx, mock.Anything, userID, comp).
		Return(nil).
		Once()

	err := s.useCase.Execute(s.ctx, input.DeleteDraftInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.NoError(err)
}

func (s *DeleteDraftBudgetSuite) TestExecute_DeleteRepositoryError() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()

	draft := entities.NewBudget(userID, comp, 100000, now)

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(draft, nil).
		Once()

	s.repo.EXPECT().
		DeleteDraft(s.ctx, mock.Anything, userID, comp).
		Return(errors.New("db error")).
		Once()

	err := s.useCase.Execute(s.ctx, input.DeleteDraftInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.Error(err)
}
