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

type EditBudgetTotalSuite struct {
	suite.Suite
	ctx     context.Context
	obs     observability.Observability
	factory *mockInterfaces.RepositoryFactory
	repo    *mockInterfaces.BudgetRepository
	uow     *uowMocks.UnitOfWorkBudget
	useCase *EditBudgetTotal
}

func TestEditBudgetTotalSuite(t *testing.T) {
	suite.Run(t, new(EditBudgetTotalSuite))
}

func (s *EditBudgetTotalSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewBudgetRepository(s.T())
	s.factory.EXPECT().BudgetRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkBudget(s.T())
	s.useCase = NewEditBudgetTotal(s.factory, s.uow, s.obs)
}

func (s *EditBudgetTotalSuite) TestExecute_Success() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	budget := activeBudgetWithAllocations(userID, comp, 100000)

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(budget, nil).
		Once()

	s.repo.EXPECT().
		Activate(mock.Anything, mock.Anything).
		Return(nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.EditBudgetTotalInput{
		UserID:     userID.String(),
		Competence: "2026-06",
		TotalCents: 200000,
	})

	s.NoError(err)
	s.Equal("active", result.State)
	s.Equal(int64(200000), result.TotalCents)
	sum := int64(0)
	for _, a := range result.Allocations {
		sum += a.PlannedCents
	}
	s.Equal(int64(200000), sum)
}

func (s *EditBudgetTotalSuite) TestExecute_InvalidUserID() {
	_, err := s.useCase.Execute(s.ctx, input.EditBudgetTotalInput{
		UserID:     "not-a-uuid",
		Competence: "2026-06",
		TotalCents: 200000,
	})

	s.Error(err)
}

func (s *EditBudgetTotalSuite) TestExecute_InvalidTotalCents() {
	_, err := s.useCase.Execute(s.ctx, input.EditBudgetTotalInput{
		UserID:     uuid.New().String(),
		Competence: "2026-06",
		TotalCents: 0,
	})

	s.ErrorIs(err, input.ErrInputInvalidTotalCents)
}

func (s *EditBudgetTotalSuite) TestExecute_BudgetNotFound() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.EditBudgetTotalInput{
		UserID:     userID.String(),
		Competence: "2026-06",
		TotalCents: 200000,
	})

	s.ErrorIs(err, interfaces.ErrBudgetNotFound)
}

func (s *EditBudgetTotalSuite) TestExecute_BudgetNotActive() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()
	draft := entities.HydrateBudget(
		uuid.New(), userID, comp, 100000,
		entities.BudgetStateDraft,
		nil, false, nil, now, now,
	)

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(draft, nil).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.EditBudgetTotalInput{
		UserID:     userID.String(),
		Competence: "2026-06",
		TotalCents: 200000,
	})

	s.ErrorIs(err, entities.ErrBudgetNotActive)
}

func (s *EditBudgetTotalSuite) TestExecute_RepositoryError() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	budget := activeBudgetWithAllocations(userID, comp, 100000)

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(budget, nil).
		Once()

	s.repo.EXPECT().
		Activate(mock.Anything, mock.Anything).
		Return(errors.New("db error")).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.EditBudgetTotalInput{
		UserID:     userID.String(),
		Competence: "2026-06",
		TotalCents: 200000,
	})

	s.Error(err)
}
