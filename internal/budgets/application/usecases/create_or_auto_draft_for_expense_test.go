package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type CreateOrAutoDraftForExpenseSuite struct {
	suite.Suite
	ctx     context.Context
	factory *mockInterfaces.RepositoryFactory
	repo    *mockInterfaces.BudgetRepository
	useCase *CreateOrAutoDraftForExpense
}

func TestCreateOrAutoDraftForExpenseSuite(t *testing.T) {
	suite.Run(t, new(CreateOrAutoDraftForExpenseSuite))
}

func (s *CreateOrAutoDraftForExpenseSuite) SetupTest() {
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewBudgetRepository(s.T())
	s.factory.EXPECT().BudgetRepository(mock.Anything).Return(s.repo).Maybe()
	s.useCase = NewCreateOrAutoDraftForExpense(s.factory)
}

func (s *CreateOrAutoDraftForExpenseSuite) TestEnsureExists_BudgetAlreadyExists_NoCreate() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()

	existing := entities.NewBudget(userID, comp, 100000, now)

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(existing, nil).
		Once()

	err := s.useCase.EnsureExists(s.ctx, nil, userID, comp, now)

	s.NoError(err)
}

func (s *CreateOrAutoDraftForExpenseSuite) TestEnsureExists_BudgetNotFound_CreatesDraft() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	s.repo.EXPECT().
		CreateDraft(mock.Anything, mock.MatchedBy(func(b entities.Budget) bool {
			return b.AutoDraft() && b.UserID() == userID
		})).
		Return(nil).
		Once()

	err := s.useCase.EnsureExists(s.ctx, nil, userID, comp, now)

	s.NoError(err)
}

func (s *CreateOrAutoDraftForExpenseSuite) TestEnsureExists_BudgetNotFound_ConflictOnCreate_Idempotent() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	s.repo.EXPECT().
		CreateDraft(mock.Anything, mock.Anything).
		Return(interfaces.ErrBudgetConflict).
		Once()

	err := s.useCase.EnsureExists(s.ctx, nil, userID, comp, now)

	s.NoError(err)
}

func (s *CreateOrAutoDraftForExpenseSuite) TestEnsureExists_GetError_Propagates() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(entities.Budget{}, errors.New("connection error")).
		Once()

	err := s.useCase.EnsureExists(s.ctx, nil, userID, comp, now)

	s.Error(err)
}

func (s *CreateOrAutoDraftForExpenseSuite) TestEnsureExists_CreateError_Propagates() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	s.repo.EXPECT().
		CreateDraft(mock.Anything, mock.Anything).
		Return(errors.New("db failure")).
		Once()

	err := s.useCase.EnsureExists(s.ctx, nil, userID, comp, now)

	s.Error(err)
}
