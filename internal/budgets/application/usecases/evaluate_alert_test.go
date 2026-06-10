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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type EvaluateAlertSuite struct {
	suite.Suite
	ctx             context.Context
	expenses        *mockInterfaces.ExpenseRepository
	budgets         *mockInterfaces.BudgetRepository
	thresholdStates *mockInterfaces.ThresholdStateRepository
	alerts          *mockInterfaces.AlertRepository
	uow             *uowMocks.UnitOfWorkVoid
	useCase         *usecases.EvaluateAlert
}

func TestEvaluateAlertSuite(t *testing.T) {
	suite.Run(t, new(EvaluateAlertSuite))
}

func (s *EvaluateAlertSuite) SetupTest() {
	s.ctx = context.Background()
	s.expenses = mockInterfaces.NewExpenseRepository(s.T())
	s.budgets = mockInterfaces.NewBudgetRepository(s.T())
	s.thresholdStates = mockInterfaces.NewThresholdStateRepository(s.T())
	s.alerts = mockInterfaces.NewAlertRepository(s.T())
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())
	s.useCase = usecases.NewEvaluateAlert(
		s.expenses,
		s.budgets,
		s.thresholdStates,
		s.alerts,
		s.uow,
		noop.NewProvider(),
	)
}

func buildActiveAlertBudget(userID uuid.UUID, comp valueobjects.Competence, total int64, rootSlug valueobjects.RootSlug, basisPoints int) entities.Budget {
	now := time.Now().UTC()
	plannedCents := total * int64(basisPoints) / 10000
	b := entities.HydrateBudget(
		uuid.New(), userID, comp, total,
		entities.BudgetStateActive,
		&now,
		false,
		[]entities.Allocation{
			entities.NewAllocation(uuid.New(), rootSlug, basisPoints, plannedCents),
		},
		now, now,
	)
	return b
}

func buildEvalInput(userID uuid.UUID, comp valueobjects.Competence, cutoff valueobjects.Competence, rootSlug valueobjects.RootSlug) usecases.EvaluateAlertInput {
	return usecases.EvaluateAlertInput{
		EventID:            uuid.New().String(),
		CommittedAt:        time.Now().UTC(),
		CutoffCompetenceBR: cutoff,
		UserID:             userID,
		Competence:         comp,
		RootSlug:           rootSlug,
	}
}

func (s *EvaluateAlertSuite) TestExecute_BudgetNotFound_NoOp() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	cutoff, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo

	s.expenses.EXPECT().
		SumByRoot(s.ctx, mock.Anything, userID, comp).
		Return(map[valueobjects.RootSlug]int64{slug: 8500}, nil).
		Once()

	s.budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	err := s.useCase.Execute(s.ctx, buildEvalInput(userID, comp, cutoff, slug))

	s.NoError(err)
}

func (s *EvaluateAlertSuite) TestExecute_BudgetNotActive_NoOp() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	cutoff, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo
	now := time.Now().UTC()

	draftBudget := entities.HydrateBudget(
		uuid.New(), userID, comp, 10000,
		entities.BudgetStateDraft,
		nil, false, nil, now, now,
	)

	s.expenses.EXPECT().
		SumByRoot(s.ctx, mock.Anything, userID, comp).
		Return(map[valueobjects.RootSlug]int64{slug: 8500}, nil).
		Once()

	s.budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(draftBudget, nil).
		Once()

	err := s.useCase.Execute(s.ctx, buildEvalInput(userID, comp, cutoff, slug))

	s.NoError(err)
}

func (s *EvaluateAlertSuite) TestExecute_NoPlannedCents_NoOp() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	cutoff, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo
	otherSlug := valueobjects.RootSlugConhecimento

	now := time.Now().UTC()
	budgetNoAlloc := entities.HydrateBudget(
		uuid.New(), userID, comp, 10000,
		entities.BudgetStateActive,
		&now, false,
		[]entities.Allocation{
			entities.NewAllocation(uuid.New(), otherSlug, 10000, 0),
		},
		now, now,
	)

	s.expenses.EXPECT().
		SumByRoot(s.ctx, mock.Anything, userID, comp).
		Return(map[valueobjects.RootSlug]int64{slug: 8500}, nil).
		Once()

	s.budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(budgetNoAlloc, nil).
		Once()

	err := s.useCase.Execute(s.ctx, buildEvalInput(userID, comp, cutoff, slug))

	s.NoError(err)
}

func (s *EvaluateAlertSuite) TestExecute_Delivered_CrossesThreshold80() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	cutoff, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo

	budget := buildActiveAlertBudget(userID, comp, 10000, slug, 10000)

	s.expenses.EXPECT().
		SumByRoot(s.ctx, mock.Anything, userID, comp).
		Return(map[valueobjects.RootSlug]int64{slug: 8000}, nil).
		Once()

	s.budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(budget, nil).
		Once()

	s.thresholdStates.EXPECT().
		GetCurrentlyCrossed(s.ctx, mock.Anything, userID, comp, slug).
		Return(map[valueobjects.Threshold]bool{valueobjects.Threshold80: false, valueobjects.Threshold100: false}, nil).
		Once()

	key80 := entities.ThresholdKey{
		UserID:     userID,
		Competence: comp,
		RootSlug:   slug,
		Threshold:  valueobjects.Threshold80,
	}
	key100 := entities.ThresholdKey{
		UserID:     userID,
		Competence: comp,
		RootSlug:   slug,
		Threshold:  valueobjects.Threshold100,
	}

	s.thresholdStates.EXPECT().
		UpsertIfTransition(s.ctx, mock.Anything, key80, true, mock.Anything).
		Return(true, nil).
		Once()

	s.thresholdStates.EXPECT().
		UpsertIfTransition(s.ctx, mock.Anything, key100, false, mock.Anything).
		Return(false, nil).
		Once()

	s.alerts.EXPECT().
		CountDelivered(s.ctx, mock.Anything, key80).
		Return(int64(0), nil).
		Once()

	s.alerts.EXPECT().
		Insert(s.ctx, mock.Anything, mock.MatchedBy(func(a entities.Alert) bool {
			return a.State() == entities.AlertStateDelivered && a.Threshold() == valueobjects.Threshold80
		})).
		Return(nil).
		Once()

	err := s.useCase.Execute(s.ctx, buildEvalInput(userID, comp, cutoff, slug))

	s.NoError(err)
}

func (s *EvaluateAlertSuite) TestExecute_SuppressedRetroactive_OlderCompetence() {
	userID := uuid.New()
	oldComp, _ := valueobjects.NewCompetence("2026-04")
	cutoff, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo

	budget := buildActiveAlertBudget(userID, oldComp, 10000, slug, 10000)

	s.expenses.EXPECT().
		SumByRoot(s.ctx, mock.Anything, userID, oldComp).
		Return(map[valueobjects.RootSlug]int64{slug: 8000}, nil).
		Once()

	s.budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, oldComp).
		Return(budget, nil).
		Once()

	s.thresholdStates.EXPECT().
		GetCurrentlyCrossed(s.ctx, mock.Anything, userID, oldComp, slug).
		Return(map[valueobjects.Threshold]bool{valueobjects.Threshold80: false, valueobjects.Threshold100: false}, nil).
		Once()

	key80 := entities.ThresholdKey{
		UserID:     userID,
		Competence: oldComp,
		RootSlug:   slug,
		Threshold:  valueobjects.Threshold80,
	}
	key100 := entities.ThresholdKey{
		UserID:     userID,
		Competence: oldComp,
		RootSlug:   slug,
		Threshold:  valueobjects.Threshold100,
	}

	s.thresholdStates.EXPECT().
		UpsertIfTransition(s.ctx, mock.Anything, key80, true, mock.Anything).
		Return(true, nil).
		Once()

	s.thresholdStates.EXPECT().
		UpsertIfTransition(s.ctx, mock.Anything, key100, false, mock.Anything).
		Return(false, nil).
		Once()

	s.alerts.EXPECT().
		Insert(s.ctx, mock.Anything, mock.MatchedBy(func(a entities.Alert) bool {
			return a.State() == entities.AlertStateSuppressedRetroactive && a.Threshold() == valueobjects.Threshold80
		})).
		Return(nil).
		Once()

	in := buildEvalInput(userID, oldComp, cutoff, slug)
	err := s.useCase.Execute(s.ctx, in)

	s.NoError(err)
}

func (s *EvaluateAlertSuite) TestExecute_SuppressedStale_WasCrossedNotAnymore() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	cutoff, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo

	budget := buildActiveAlertBudget(userID, comp, 10000, slug, 10000)

	s.expenses.EXPECT().
		SumByRoot(s.ctx, mock.Anything, userID, comp).
		Return(map[valueobjects.RootSlug]int64{slug: 100}, nil).
		Once()

	s.budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(budget, nil).
		Once()

	s.thresholdStates.EXPECT().
		GetCurrentlyCrossed(s.ctx, mock.Anything, userID, comp, slug).
		Return(map[valueobjects.Threshold]bool{valueobjects.Threshold80: true, valueobjects.Threshold100: false}, nil).
		Once()

	key80 := entities.ThresholdKey{
		UserID:     userID,
		Competence: comp,
		RootSlug:   slug,
		Threshold:  valueobjects.Threshold80,
	}
	key100 := entities.ThresholdKey{
		UserID:     userID,
		Competence: comp,
		RootSlug:   slug,
		Threshold:  valueobjects.Threshold100,
	}

	s.thresholdStates.EXPECT().
		UpsertIfTransition(s.ctx, mock.Anything, key80, false, mock.Anything).
		Return(false, nil).
		Once()

	s.thresholdStates.EXPECT().
		UpsertIfTransition(s.ctx, mock.Anything, key100, false, mock.Anything).
		Return(false, nil).
		Once()

	err := s.useCase.Execute(s.ctx, buildEvalInput(userID, comp, cutoff, slug))

	s.NoError(err)
}

func (s *EvaluateAlertSuite) TestExecute_RateLimited_ExceedsMaxDelivered() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	cutoff, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo

	budget := buildActiveAlertBudget(userID, comp, 10000, slug, 10000)

	s.expenses.EXPECT().
		SumByRoot(s.ctx, mock.Anything, userID, comp).
		Return(map[valueobjects.RootSlug]int64{slug: 8000}, nil).
		Once()

	s.budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(budget, nil).
		Once()

	s.thresholdStates.EXPECT().
		GetCurrentlyCrossed(s.ctx, mock.Anything, userID, comp, slug).
		Return(map[valueobjects.Threshold]bool{valueobjects.Threshold80: false, valueobjects.Threshold100: false}, nil).
		Once()

	key80 := entities.ThresholdKey{
		UserID:     userID,
		Competence: comp,
		RootSlug:   slug,
		Threshold:  valueobjects.Threshold80,
	}
	key100 := entities.ThresholdKey{
		UserID:     userID,
		Competence: comp,
		RootSlug:   slug,
		Threshold:  valueobjects.Threshold100,
	}

	s.thresholdStates.EXPECT().
		UpsertIfTransition(s.ctx, mock.Anything, key80, true, mock.Anything).
		Return(true, nil).
		Once()

	s.thresholdStates.EXPECT().
		UpsertIfTransition(s.ctx, mock.Anything, key100, false, mock.Anything).
		Return(false, nil).
		Once()

	s.alerts.EXPECT().
		CountDelivered(s.ctx, mock.Anything, key80).
		Return(int64(10), nil).
		Once()

	s.alerts.EXPECT().
		Insert(s.ctx, mock.Anything, mock.MatchedBy(func(a entities.Alert) bool {
			return a.State() == entities.AlertStateRateLimited && a.Threshold() == valueobjects.Threshold80
		})).
		Return(nil).
		Once()

	err := s.useCase.Execute(s.ctx, buildEvalInput(userID, comp, cutoff, slug))

	s.NoError(err)
}

func (s *EvaluateAlertSuite) TestExecute_BudgetRepositoryError_PropagatesError() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	cutoff, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo

	s.expenses.EXPECT().
		SumByRoot(s.ctx, mock.Anything, userID, comp).
		Return(map[valueobjects.RootSlug]int64{}, nil).
		Once()

	s.budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(entities.Budget{}, errors.New("db error")).
		Once()

	err := s.useCase.Execute(s.ctx, buildEvalInput(userID, comp, cutoff, slug))

	s.Error(err)
}

func (s *EvaluateAlertSuite) TestExecute_ExpenseRepositoryError_PropagatesError() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	cutoff, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo

	s.expenses.EXPECT().
		SumByRoot(s.ctx, mock.Anything, userID, comp).
		Return(nil, errors.New("sum error")).
		Once()

	err := s.useCase.Execute(s.ctx, buildEvalInput(userID, comp, cutoff, slug))

	s.Error(err)
}

func (s *EvaluateAlertSuite) TestExecute_Delivered_BothThresholds100() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	cutoff, _ := valueobjects.NewCompetence("2026-06")
	slug := valueobjects.RootSlugCustoFixo

	budget := buildActiveAlertBudget(userID, comp, 10000, slug, 10000)

	s.expenses.EXPECT().
		SumByRoot(s.ctx, mock.Anything, userID, comp).
		Return(map[valueobjects.RootSlug]int64{slug: 10000}, nil).
		Once()

	s.budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, userID, comp).
		Return(budget, nil).
		Once()

	s.thresholdStates.EXPECT().
		GetCurrentlyCrossed(s.ctx, mock.Anything, userID, comp, slug).
		Return(map[valueobjects.Threshold]bool{valueobjects.Threshold80: false, valueobjects.Threshold100: false}, nil).
		Once()

	key80 := entities.ThresholdKey{
		UserID:     userID,
		Competence: comp,
		RootSlug:   slug,
		Threshold:  valueobjects.Threshold80,
	}
	key100 := entities.ThresholdKey{
		UserID:     userID,
		Competence: comp,
		RootSlug:   slug,
		Threshold:  valueobjects.Threshold100,
	}

	s.thresholdStates.EXPECT().
		UpsertIfTransition(s.ctx, mock.Anything, key80, true, mock.Anything).
		Return(true, nil).
		Once()

	s.thresholdStates.EXPECT().
		UpsertIfTransition(s.ctx, mock.Anything, key100, true, mock.Anything).
		Return(true, nil).
		Once()

	s.alerts.EXPECT().
		CountDelivered(s.ctx, mock.Anything, key80).
		Return(int64(0), nil).
		Once()

	s.alerts.EXPECT().
		CountDelivered(s.ctx, mock.Anything, key100).
		Return(int64(0), nil).
		Once()

	s.alerts.EXPECT().
		Insert(s.ctx, mock.Anything, mock.MatchedBy(func(a entities.Alert) bool {
			return a.State() == entities.AlertStateDelivered && a.Threshold() == valueobjects.Threshold80
		})).
		Return(nil).
		Once()

	s.alerts.EXPECT().
		Insert(s.ctx, mock.Anything, mock.MatchedBy(func(a entities.Alert) bool {
			return a.State() == entities.AlertStateDelivered && a.Threshold() == valueobjects.Threshold100
		})).
		Return(nil).
		Once()

	err := s.useCase.Execute(s.ctx, buildEvalInput(userID, comp, cutoff, slug))

	s.NoError(err)
}
