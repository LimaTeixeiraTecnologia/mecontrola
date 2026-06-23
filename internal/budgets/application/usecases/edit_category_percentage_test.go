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

type EditCategoryPercentageSuite struct {
	suite.Suite
	ctx       context.Context
	obs       observability.Observability
	factory   *mockInterfaces.RepositoryFactory
	repo      *mockInterfaces.BudgetRepository
	publisher *fakeBudgetActivatedPublisher
	uow       *uowMocks.UnitOfWorkBudget
	useCase   *EditCategoryPercentage
}

func TestEditCategoryPercentageSuite(t *testing.T) {
	suite.Run(t, new(EditCategoryPercentageSuite))
}

func (s *EditCategoryPercentageSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewBudgetRepository(s.T())
	s.publisher = &fakeBudgetActivatedPublisher{}
	s.factory.EXPECT().BudgetRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkBudget(s.T())
	s.useCase = NewEditCategoryPercentage(s.factory, s.publisher, s.uow, s.obs)
}

func activeBudgetWithAllocations(userID uuid.UUID, comp valueobjects.Competence, total int64) entities.Budget {
	now := time.Now().UTC()
	slug1, _ := valueobjects.ParseRootSlug("expense.custo_fixo")
	slug2, _ := valueobjects.ParseRootSlug("expense.conhecimento")
	id := uuid.New()
	allocs := []entities.Allocation{
		entities.NewAllocation(id, slug1, 6000, total*6000/10000),
		entities.NewAllocation(id, slug2, 4000, total*4000/10000),
	}
	return entities.HydrateBudget(
		id, userID, comp, total,
		entities.BudgetStateActive,
		&now,
		false,
		allocs,
		now,
		now,
	)
}

func (s *EditCategoryPercentageSuite) TestExecute_Success() {
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

	result, err := s.useCase.Execute(s.ctx, input.EditCategoryPercentageInput{
		UserID:     userID.String(),
		Competence: "2026-06",
		RootSlug:   "expense.custo_fixo",
		Percentage: 50,
	})

	s.NoError(err)
	s.Equal("active", result.State)
	s.Len(result.Allocations, 2)
	sum := 0
	for _, a := range result.Allocations {
		sum += a.BasisPoints
	}
	s.Equal(10000, sum)
	s.Equal(1, s.publisher.calls)
}

func (s *EditCategoryPercentageSuite) TestExecute_InvalidUserID() {
	_, err := s.useCase.Execute(s.ctx, input.EditCategoryPercentageInput{
		UserID:     "not-a-uuid",
		Competence: "2026-06",
		RootSlug:   "expense.custo_fixo",
		Percentage: 50,
	})

	s.Error(err)
	s.Equal(0, s.publisher.calls)
}

func (s *EditCategoryPercentageSuite) TestExecute_PercentageOutOfRange() {
	_, err := s.useCase.Execute(s.ctx, input.EditCategoryPercentageInput{
		UserID:     uuid.New().String(),
		Competence: "2026-06",
		RootSlug:   "expense.custo_fixo",
		Percentage: 150,
	})

	s.ErrorIs(err, input.ErrInputPercentageRange)
	s.Equal(0, s.publisher.calls)
}

func (s *EditCategoryPercentageSuite) TestExecute_BudgetNotFound() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.EditCategoryPercentageInput{
		UserID:     userID.String(),
		Competence: "2026-06",
		RootSlug:   "expense.custo_fixo",
		Percentage: 50,
	})

	s.ErrorIs(err, interfaces.ErrBudgetNotFound)
	s.Equal(0, s.publisher.calls)
}

func (s *EditCategoryPercentageSuite) TestExecute_BudgetNotActive() {
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

	_, err := s.useCase.Execute(s.ctx, input.EditCategoryPercentageInput{
		UserID:     userID.String(),
		Competence: "2026-06",
		RootSlug:   "expense.custo_fixo",
		Percentage: 50,
	})

	s.ErrorIs(err, entities.ErrBudgetNotActive)
	s.Equal(0, s.publisher.calls)
}

func (s *EditCategoryPercentageSuite) TestExecute_RepositoryError() {
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

	_, err := s.useCase.Execute(s.ctx, input.EditCategoryPercentageInput{
		UserID:     userID.String(),
		Competence: "2026-06",
		RootSlug:   "expense.custo_fixo",
		Percentage: 50,
	})

	s.Error(err)
	s.Equal(0, s.publisher.calls)
}
