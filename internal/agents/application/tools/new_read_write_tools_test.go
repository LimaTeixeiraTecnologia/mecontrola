package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func TestBuildCancelPlanInfoTool(t *testing.T) {
	handle := BuildCancelPlanInfoTool()
	assert.Equal(t, "cancel_plan_info", handle.ID())

	out, verbatimText, err := handle.Invoke(context.Background(), []byte(`{}`))
	require.NoError(t, err)

	var result CancelPlanInfoOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, messages.CancelPlanInfo(), result.Message)
	assert.Equal(t, result.Message, verbatimText)
}

func TestBuildSupportInfoTool(t *testing.T) {
	handle := BuildSupportInfoTool()
	assert.Equal(t, "support_info", handle.ID())

	out, verbatimText, err := handle.Invoke(context.Background(), []byte(`{}`))
	require.NoError(t, err)

	var result SupportInfoOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, messages.SupportInfo(), result.Message)
	assert.Equal(t, result.Message, verbatimText)
}

func TestBuildEditGoalTool_Started(t *testing.T) {
	engine := &fakeGoalEditEngine{
		startResult: wf.RunResult[workflows.GoalEditState]{
			State: workflows.GoalEditState{ResponseText: "Qual é o seu objetivo financeiro? 🎯"},
		},
	}
	handle := BuildEditGoalTool(engine, fakeGoalEditDef())
	assert.Equal(t, "edit_goal", handle.ID())

	out, _, err := handle.Invoke(inboundCtx(), []byte(`{}`))
	require.NoError(t, err)

	var result EditGoalOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, editGoalOutcomeStarted, result.Outcome)
	assert.NotEmpty(t, result.ConfirmationPrompt)
	assert.True(t, engine.startCalled)
}

func TestBuildEditGoalTool_AlreadyExists(t *testing.T) {
	engine := &fakeGoalEditEngine{startErr: wf.ErrRunAlreadyExists}
	handle := BuildEditGoalTool(engine, fakeGoalEditDef())

	out, _, err := handle.Invoke(inboundCtx(), []byte(`{}`))
	require.NoError(t, err)

	var result EditGoalOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, editGoalOutcomePendingExists, result.Outcome)
}

func TestBuildEditGoalTool_MissingIdentity(t *testing.T) {
	engine := &fakeGoalEditEngine{}
	handle := BuildEditGoalTool(engine, fakeGoalEditDef())

	_, _, err := handle.Invoke(context.Background(), []byte(`{}`))
	assert.Error(t, err)
	assert.False(t, engine.startCalled)
}

func TestBuildEditBudgetTotalTool_Started(t *testing.T) {
	engine := newFakeBudgetManageEngine()
	handle := BuildEditBudgetTotalTool(engine, fakeBudgetManageDef())
	assert.Equal(t, "edit_budget_total", handle.ID())

	argsJSON, _ := json.Marshal(EditBudgetTotalInput{MonthRefKind: "current"})
	out, _, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result EditBudgetTotalOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, editBudgetTotalOutcomeStarted, result.Outcome)
	assert.True(t, engine.startCalled)
	assert.Equal(t, workflows.BudgetManageOpEditTotal, engine.lastState.Operation)
}

func TestBuildEditBudgetTotalTool_Clarify(t *testing.T) {
	engine := newFakeBudgetManageEngine()
	handle := BuildEditBudgetTotalTool(engine, fakeBudgetManageDef())

	argsJSON, _ := json.Marshal(EditBudgetTotalInput{MonthRefKind: "named_without_year", Month: 6})
	out, _, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result EditBudgetTotalOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, editBudgetTotalOutcomeClarify, result.Outcome)
	assert.False(t, engine.startCalled)
}

func TestBuildCategoryDetailTool_GeneralSummary(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)
	ledger := imocks.NewTransactionsLedger(t)
	reader := imocks.NewCategoriesReader(t)

	total := int64(500000)
	spentTotal := int64(100000)
	planner.EXPECT().
		GetMonthlySummary(mock.Anything, testUserID, mock.AnythingOfType("string")).
		Return(interfaces.BudgetSummary{
			TotalCents:        &total,
			TotalPlannedCents: &total,
			TotalSpentCents:   spentTotal,
			Allocations: []interfaces.AllocationSummary{
				{RootSlug: "custo-fixo", PlannedCents: &total, SpentCents: spentTotal},
			},
		}, nil).Once()
	reader.EXPECT().
		ListCategories(mock.Anything, testUserID).
		Return([]interfaces.Category{
			{Slug: "custo-fixo", Name: "Custo Fixo"},
		}, nil).Once()

	handle := BuildCategoryDetailTool(planner, ledger, reader)
	assert.Equal(t, "category_detail", handle.ID())

	argsJSON, _ := json.Marshal(CategoryDetailInput{MonthRefKind: "current"})
	out, verbatimText, err := handle.Invoke(identityCtx("wamid-cat-1", 0), argsJSON)
	require.NoError(t, err)

	var result CategoryDetailOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, categoryDetailOutcomeOK, result.Outcome)
	assert.NotEmpty(t, result.Message)
	assert.Equal(t, result.Message, verbatimText)
	assert.Contains(t, result.Message, "Custo Fixo")
	assert.NotContains(t, result.Message, "custo-fixo")
}

func TestBuildCategoryDetailTool_SpecificCategory(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)
	ledger := imocks.NewTransactionsLedger(t)
	reader := imocks.NewCategoriesReader(t)

	rootID := uuid.New()
	planned := int64(200000)
	spent := int64(50000)
	planner.EXPECT().
		GetMonthlySummary(mock.Anything, testUserID, mock.AnythingOfType("string")).
		Return(interfaces.BudgetSummary{
			Allocations: []interfaces.AllocationSummary{
				{RootSlug: "custo-fixo", PlannedCents: &planned, SpentCents: spent},
			},
		}, nil).Once()
	reader.EXPECT().
		ListCategories(mock.Anything, testUserID).
		Return([]interfaces.Category{
			{ID: rootID, Slug: "custo-fixo", Name: "Custo Fixo"},
		}, nil).Once()
	ledger.EXPECT().
		ListMonthlyEntries(mock.Anything, testUserID, mock.AnythingOfType("string"), "", categoryDetailEntriesLimit).
		Return([]interfaces.MonthlyEntry{}, nil).Once()

	handle := BuildCategoryDetailTool(planner, ledger, reader)

	argsJSON, _ := json.Marshal(CategoryDetailInput{Category: "custo fixo", MonthRefKind: "current"})
	out, _, err := handle.Invoke(identityCtx("wamid-cat-2", 0), argsJSON)
	require.NoError(t, err)

	var result CategoryDetailOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, categoryDetailOutcomeOK, result.Outcome)
	assert.Contains(t, result.Message, "Custo Fixo")
}

func TestBuildCategoryDetailTool_CategoryNotFound(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)
	ledger := imocks.NewTransactionsLedger(t)
	reader := imocks.NewCategoriesReader(t)

	planner.EXPECT().
		GetMonthlySummary(mock.Anything, testUserID, mock.AnythingOfType("string")).
		Return(interfaces.BudgetSummary{}, nil).Once()
	reader.EXPECT().
		ListCategories(mock.Anything, testUserID).
		Return([]interfaces.Category{}, nil).Once()

	handle := BuildCategoryDetailTool(planner, ledger, reader)

	argsJSON, _ := json.Marshal(CategoryDetailInput{Category: "inexistente", MonthRefKind: "current"})
	out, _, err := handle.Invoke(identityCtx("wamid-cat-3", 0), argsJSON)
	require.NoError(t, err)

	var result CategoryDetailOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, categoryDetailOutcomeClarify, result.Outcome)
	assert.NotEmpty(t, result.ClarifyPrompt)
}

func TestBuildCategoryDetailTool_BudgetNotFound(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)
	ledger := imocks.NewTransactionsLedger(t)
	reader := imocks.NewCategoriesReader(t)

	planner.EXPECT().
		GetMonthlySummary(mock.Anything, testUserID, mock.AnythingOfType("string")).
		Return(interfaces.BudgetSummary{}, interfaces.ErrBudgetNotFound).Once()

	handle := BuildCategoryDetailTool(planner, ledger, reader)

	argsJSON, _ := json.Marshal(CategoryDetailInput{MonthRefKind: "current"})
	out, _, err := handle.Invoke(identityCtx("wamid-cat-4", 0), argsJSON)
	require.NoError(t, err)

	var result CategoryDetailOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, categoryDetailOutcomeNotFound, result.Outcome)
	assert.NotEmpty(t, result.OfferCreatePrompt)
}

type fakeGoalEditEngine struct {
	startResult wf.RunResult[workflows.GoalEditState]
	startErr    error
	startCalled bool
	lastState   workflows.GoalEditState
}

func (f *fakeGoalEditEngine) Start(_ context.Context, _ wf.Definition[workflows.GoalEditState], _ string, initial workflows.GoalEditState) (wf.RunResult[workflows.GoalEditState], error) {
	f.startCalled = true
	f.lastState = initial
	return f.startResult, f.startErr
}

func (f *fakeGoalEditEngine) Resume(_ context.Context, _ wf.Definition[workflows.GoalEditState], _ string, _ []byte) (wf.RunResult[workflows.GoalEditState], error) {
	return wf.RunResult[workflows.GoalEditState]{}, nil
}

func (f *fakeGoalEditEngine) LoadLatestState(_ context.Context, _ wf.Definition[workflows.GoalEditState], _ string) (workflows.GoalEditState, wf.Snapshot, bool, error) {
	return workflows.GoalEditState{}, wf.Snapshot{}, false, nil
}

func fakeGoalEditDef() wf.Definition[workflows.GoalEditState] {
	return wf.Definition[workflows.GoalEditState]{
		ID:      workflows.GoalEditWorkflowID,
		Durable: true,
	}
}
