package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	ifmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type GetMonthlySummarySuite struct {
	suite.Suite
	ctx      context.Context
	obs      observability.Observability
	uc       *GetMonthlySummary
	factory  *ifmocks.RepositoryFactory
	budgets  *ifmocks.BudgetRepository
	expenses *ifmocks.ExpenseRepository
}

func (s *GetMonthlySummarySuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.factory = ifmocks.NewRepositoryFactory(s.T())
	s.budgets = ifmocks.NewBudgetRepository(s.T())
	s.expenses = ifmocks.NewExpenseRepository(s.T())
	s.factory.EXPECT().BudgetRepository(mock.Anything).Return(s.budgets).Maybe()
	s.factory.EXPECT().ExpenseRepository(mock.Anything).Return(s.expenses).Maybe()
	uow := ucmocks.NewUnitOfWorkMonthlySummary(s.T())
	s.uc = NewGetMonthlySummary(s.factory, uow, s.obs)
}

func TestGetMonthlySummarySuite(t *testing.T) {
	suite.Run(t, new(GetMonthlySummarySuite))
}

func (s *GetMonthlySummarySuite) TestExecute_InvalidUserID() {
	_, err := s.uc.Execute(s.ctx, "not-a-uuid", "2025-01")
	s.ErrorIs(err, ErrGetSummaryInvalidUserID)
}

func (s *GetMonthlySummarySuite) TestExecute_InvalidCompetence() {
	_, err := s.uc.Execute(s.ctx, uuid.New().String(), "25-01")
	s.ErrorIs(err, ErrGetSummaryInvalidCompetence)
}

func (s *GetMonthlySummarySuite) TestExecute_BudgetNotFound() {
	userID := uuid.New()
	s.budgets.EXPECT().GetByUserCompetence(mock.Anything, userID, mock.Anything).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound)

	_, err := s.uc.Execute(s.ctx, userID.String(), "2025-01")
	s.ErrorIs(err, interfaces.ErrBudgetNotFound)
}

func (s *GetMonthlySummarySuite) TestExecute_AutoDraftReturnsSummaryWithNullFields() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2025-01")
	budget := entities.NewAutoDraftBudget(userID, comp, time.Now())

	s.budgets.EXPECT().GetByUserCompetence(mock.Anything, userID, mock.Anything).
		Return(budget, nil)
	s.expenses.EXPECT().SumByRoot(mock.Anything, userID, mock.Anything).
		Return(map[valueobjects.RootSlug]int64{}, nil)

	out, err := s.uc.Execute(s.ctx, userID.String(), "2025-01")
	s.NoError(err)
	s.True(out.AutoDraft)
	s.Nil(out.TotalCents)
	for _, a := range out.Allocations {
		s.Nil(a.PlannedCents)
		s.Nil(a.PercentageSpent)
	}
}

func (s *GetMonthlySummarySuite) TestExecute_ActiveBudgetWithExpenses() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2025-01")
	now := time.Now()
	budget := entities.HydrateBudget(
		uuid.New(), userID, comp, 100000,
		entities.BudgetStateActive, &now, false,
		[]entities.Allocation{
			entities.NewAllocation(uuid.New(), valueobjects.RootSlugCustoFixo, 5000, 50000),
		},
		now, now,
	)

	s.budgets.EXPECT().GetByUserCompetence(mock.Anything, userID, mock.Anything).
		Return(budget, nil)
	s.expenses.EXPECT().SumByRoot(mock.Anything, userID, mock.Anything).
		Return(map[valueobjects.RootSlug]int64{valueobjects.RootSlugCustoFixo: 25000}, nil)

	out, err := s.uc.Execute(s.ctx, userID.String(), "2025-01")
	s.NoError(err)
	s.Equal("active", out.State)
	s.Require().Len(out.Allocations, 5)
	custo := out.Allocations[0]
	s.Equal(valueobjects.RootSlugCustoFixo.String(), custo.RootSlug)
	s.Equal(int64(25000), custo.SpentCents)
	s.NotNil(custo.PercentageSpent)
	s.InDelta(50.0, *custo.PercentageSpent, 0.001)
	s.Equal(int64(25000), out.TotalSpentCents)
	s.Require().NotNil(out.TotalPlannedCents)
	s.Equal(int64(50000), *out.TotalPlannedCents)
	s.Require().NotNil(out.PercentageTotal)
	s.InDelta(50.0, *out.PercentageTotal, 0.001)
}

func (s *GetMonthlySummarySuite) TestExecute_AutoDraftShowsAllRootsWithSpentFromExpenses() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2025-01")
	budget := entities.NewAutoDraftBudget(userID, comp, time.Now())

	spent := map[valueobjects.RootSlug]int64{
		valueobjects.RootSlugCustoFixo:           12000,
		valueobjects.RootSlugConhecimento:        3400,
		valueobjects.RootSlugPrazeres:            5600,
		valueobjects.RootSlugMetas:               0,
		valueobjects.RootSlugLiberdadeFinanceira: 7800,
	}

	s.budgets.EXPECT().GetByUserCompetence(mock.Anything, userID, mock.Anything).
		Return(budget, nil)
	s.expenses.EXPECT().SumByRoot(mock.Anything, userID, mock.Anything).
		Return(spent, nil)

	out, err := s.uc.Execute(s.ctx, userID.String(), "2025-01")
	s.NoError(err)
	s.True(out.AutoDraft)
	s.Nil(out.TotalCents)
	s.Require().Len(out.Allocations, 5)

	canonical := valueobjects.CanonicalOrder()
	for i, root := range canonical {
		s.Equal(root.String(), out.Allocations[i].RootSlug)
		s.Equal(spent[root], out.Allocations[i].SpentCents)
		s.Nil(out.Allocations[i].PlannedCents)
		s.Nil(out.Allocations[i].PercentageSpent)
	}

	s.Equal(int64(12000+3400+5600+0+7800), out.TotalSpentCents)
	s.Nil(out.TotalPlannedCents)
	s.Nil(out.PercentageTotal)
}
