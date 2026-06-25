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

type ActivateBudgetSuite struct {
	suite.Suite
	ctx     context.Context
	obs     observability.Observability
	factory *mockInterfaces.RepositoryFactory
	repo    *mockInterfaces.BudgetRepository
	uow     *uowMocks.UnitOfWorkBudget
	useCase *ActivateBudget
}

func TestActivateBudgetSuite(t *testing.T) {
	suite.Run(t, new(ActivateBudgetSuite))
}

func (s *ActivateBudgetSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewBudgetRepository(s.T())
	s.factory.EXPECT().BudgetRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkBudget(s.T())
	s.useCase = NewActivateBudget(s.factory, s.uow, s.obs)
}

func buildDraftBudgetWithAllocations(userID uuid.UUID, comp valueobjects.Competence, total int64) entities.Budget {
	now := time.Now().UTC()
	b := entities.NewBudget(userID, comp, total, now)
	slug1, _ := valueobjects.ParseRootSlug("expense.custo_fixo")
	slug2, _ := valueobjects.ParseRootSlug("expense.conhecimento")
	b.SetAllocations([]entities.Allocation{
		entities.NewAllocation(b.ID(), slug1, 6000, 0),
		entities.NewAllocation(b.ID(), slug2, 4000, 0),
	})
	return b
}

func (s *ActivateBudgetSuite) TestExecute_InvalidUserID() {
	_, err := s.useCase.Execute(s.ctx, input.ActivateBudgetInput{
		UserID:     "not-a-uuid",
		Competence: "2026-06",
	})

	s.Error(err)
}

func (s *ActivateBudgetSuite) TestExecute_InvalidCompetence() {
	_, err := s.useCase.Execute(s.ctx, input.ActivateBudgetInput{
		UserID:     uuid.New().String(),
		Competence: "bad-comp",
	})

	s.Error(err)
}

func (s *ActivateBudgetSuite) TestExecute_BudgetNotFound() {
	userID := uuid.New()

	comp, _ := valueobjects.NewCompetence("2026-06")
	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.ErrorIs(err, interfaces.ErrBudgetNotFound)
}

func (s *ActivateBudgetSuite) TestExecute_AlreadyActive() {
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

	_, err := s.useCase.Execute(s.ctx, input.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.ErrorIs(err, entities.ErrBudgetAlreadyActive)
}

func (s *ActivateBudgetSuite) TestExecute_ActivationSuccess_WithDistribution() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")

	draft := buildDraftBudgetWithAllocations(userID, comp, 10000)

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(draft, nil).
		Once()

	s.repo.EXPECT().
		Activate(mock.Anything, mock.Anything).
		Return(nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.NoError(err)
	s.Equal("active", result.State)
	s.NotNil(result.ActivatedAt)
	s.Equal(int64(10000), result.TotalCents)
	s.Len(result.Allocations, 2)
}

func (s *ActivateBudgetSuite) TestExecute_CentDistributionCorrect() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")

	draft := buildDraftBudgetWithAllocations(userID, comp, 1001)

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(draft, nil).
		Once()

	s.repo.EXPECT().
		Activate(mock.Anything, mock.Anything).
		Return(nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.NoError(err)
	totalPlanned := int64(0)
	for _, a := range result.Allocations {
		totalPlanned += a.PlannedCents
	}
	s.Equal(int64(1001), totalPlanned)
}

func (s *ActivateBudgetSuite) TestExecute_ActivateRepositoryError() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")

	draft := buildDraftBudgetWithAllocations(userID, comp, 10000)

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(draft, nil).
		Once()

	s.repo.EXPECT().
		Activate(mock.Anything, mock.Anything).
		Return(errors.New("db error")).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.Error(err)
}

func (s *ActivateBudgetSuite) TestExecute_ActivateConflict() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")

	draft := buildDraftBudgetWithAllocations(userID, comp, 10000)

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(draft, nil).
		Once()

	s.repo.EXPECT().
		Activate(mock.Anything, mock.Anything).
		Return(interfaces.ErrBudgetConflict).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.ErrorIs(err, interfaces.ErrBudgetConflict)
}

func (s *ActivateBudgetSuite) TestExecute_AllocationSumNot10000() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")

	now := time.Now().UTC()
	slug1, _ := valueobjects.ParseRootSlug("expense.custo_fixo")
	b := entities.NewBudget(userID, comp, 10000, now)
	b.SetAllocations([]entities.Allocation{
		entities.NewAllocation(b.ID(), slug1, 5000, 0),
	})

	s.repo.EXPECT().
		GetByUserCompetence(mock.Anything, userID, comp).
		Return(b, nil).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})

	s.ErrorIs(err, entities.ErrBudgetAllocationSumMustBe10000)
}
