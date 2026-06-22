package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type DeleteDraftBudgetSuite struct {
	suite.Suite
	obs     observability.Observability
	ctx     context.Context
	factory *mockInterfaces.RepositoryFactory
	repo    *mockInterfaces.BudgetRepository
	uow     *uowMocks.UnitOfWorkVoid
	useCase *DeleteDraftBudget
}

func TestDeleteDraftBudgetSuite(t *testing.T) {
	suite.Run(t, new(DeleteDraftBudgetSuite))
}

func (s *DeleteDraftBudgetSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewBudgetRepository(s.T())
	s.factory.EXPECT().BudgetRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())
	s.useCase = NewDeleteDraftBudget(s.factory, s.uow, s.obs)
}

func (s *DeleteDraftBudgetSuite) TestExecute_InvalidUserID() {
	err := s.useCase.Execute(s.ctx, input.DeleteDraftInput{
		UserID:     "not-a-uuid",
		Competence: "2026-06",
	})

	s.ErrorIs(err, ErrBudgetInvalidUserID)
}

func (s *DeleteDraftBudgetSuite) TestExecute_InvalidCompetence() {
	err := s.useCase.Execute(s.ctx, input.DeleteDraftInput{
		UserID:     uuid.New().String(),
		Competence: "bad",
	})

	s.ErrorIs(err, ErrBudgetInvalidCompetence)
}

func (s *DeleteDraftBudgetSuite) TestExecute_BudgetNotFound() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
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
		GetByUserCompetence(mock.Anything, userID, comp).
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
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(draft, nil).
		Once()

	s.repo.EXPECT().
		DeleteDraft(mock.Anything, userID, comp).
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
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(draft, nil).
		Once()

	s.repo.EXPECT().
		DeleteDraft(mock.Anything, userID, comp).
		Return(errors.New("db error")).
		Once()

	err := s.useCase.Execute(s.ctx, input.DeleteDraftInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.Error(err)
}
