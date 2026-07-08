package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type fakeConfirmEngine struct {
	startResult wf.RunResult[workflows.ConfirmState]
	startErr    error
}

func (f *fakeConfirmEngine) Start(_ context.Context, _ wf.Definition[workflows.ConfirmState], _ string, _ workflows.ConfirmState) (wf.RunResult[workflows.ConfirmState], error) {
	return f.startResult, f.startErr
}

func (f *fakeConfirmEngine) Resume(_ context.Context, _ wf.Definition[workflows.ConfirmState], _ string, _ []byte) (wf.RunResult[workflows.ConfirmState], error) {
	return wf.RunResult[workflows.ConfirmState]{}, nil
}

func fakeConfirmDef() wf.Definition[workflows.ConfirmState] {
	return wf.Definition[workflows.ConfirmState]{
		ID:      workflows.DestructiveConfirmWorkflowID,
		Durable: true,
	}
}

func inboundCtx() context.Context {
	req := agent.InboundRequest{
		ResourceID: testUserID.String(),
		ThreadID:   "thread-1",
		AgentID:    "mecontrola-agent",
		Message:    "delete",
		MessageID:  "msg-001",
	}
	return wf.WithRuntime(context.Background(), req)
}

func identityCtx(wamid string, itemSeq int) context.Context {
	return agent.WithToolInvocationContext(context.Background(), testUserID.String(), wamid, itemSeq)
}

var testUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
var testResourceID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
var testCategoryID = uuid.MustParse("00000000-0000-0000-0000-000000000003")

type fakeRegistrar struct {
	expenseResult usecases.RegisterResult
	expenseErr    error
	incomeResult  usecases.RegisterResult
	incomeErr     error
	lastExpense   usecases.RegisterExpenseCommand
	lastIncome    usecases.RegisterIncomeCommand
	expenseCalls  int
	incomeCalls   int
}

func (f *fakeRegistrar) RegisterExpense(_ context.Context, cmd usecases.RegisterExpenseCommand) (usecases.RegisterResult, error) {
	f.expenseCalls++
	f.lastExpense = cmd
	return f.expenseResult, f.expenseErr
}

func (f *fakeRegistrar) RegisterIncome(_ context.Context, cmd usecases.RegisterIncomeCommand) (usecases.RegisterResult, error) {
	f.incomeCalls++
	f.lastIncome = cmd
	return f.incomeResult, f.incomeErr
}

func TestBuildRegisterExpenseToolDelegatesAndMapsOutput(t *testing.T) {
	registrar := &fakeRegistrar{
		expenseResult: usecases.RegisterResult{ResourceID: testResourceID, Kind: "transaction", Outcome: agent.ToolOutcomeRouted},
	}

	handle := BuildRegisterExpenseTool(registrar)
	assert.Equal(t, "register_expense", handle.ID())
	assert.NotEmpty(t, handle.Description())

	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   5000,
		Description:   "Almoço",
		PaymentMethod: "debit_card",
	})
	out, err := handle.Invoke(identityCtx("wamid1", 2), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, testResourceID.String(), result.ResourceID)
	assert.Equal(t, "transaction", result.Kind)
	assert.False(t, result.IsReplay)
	assert.Equal(t, "routed", result.Outcome)

	assert.Equal(t, 1, registrar.expenseCalls)
	assert.Equal(t, testUserID, registrar.lastExpense.UserID)
	assert.Equal(t, "wamid1", registrar.lastExpense.WAMID)
	assert.Equal(t, 2, registrar.lastExpense.ItemSeq)
	assert.Equal(t, int64(5000), registrar.lastExpense.AmountCents)
	assert.Equal(t, "Almoço", registrar.lastExpense.Description)
	assert.Equal(t, "debit_card", registrar.lastExpense.PaymentMethod)
}

func TestBuildRegisterExpenseToolReplay(t *testing.T) {
	registrar := &fakeRegistrar{
		expenseResult: usecases.RegisterResult{ResourceID: testResourceID, Kind: "transaction", Outcome: agent.ToolOutcomeReplay},
	}

	handle := BuildRegisterExpenseTool(registrar)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "Almoço", PaymentMethod: "debit_card"})
	out, err := handle.Invoke(identityCtx("wamid1", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.IsReplay)
	assert.Equal(t, "replay", result.Outcome)
}

func TestBuildRegisterExpenseToolClarifyOmitsResource(t *testing.T) {
	registrar := &fakeRegistrar{
		expenseResult: usecases.RegisterResult{Outcome: agent.ToolOutcomeClarify},
	}

	handle := BuildRegisterExpenseTool(registrar)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "algo ambíguo", PaymentMethod: "pix"})
	out, err := handle.Invoke(identityCtx("wamid1", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome)
	assert.Empty(t, result.ResourceID)
	assert.False(t, result.IsReplay)
}

func TestBuildRegisterExpenseToolInvalidUserID(t *testing.T) {
	registrar := &fakeRegistrar{}
	handle := BuildRegisterExpenseTool(registrar)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "Almoço", PaymentMethod: "debit_card"})
	invalidCtx := agent.WithToolInvocationContext(context.Background(), "not-a-uuid", "wamid1", 0)
	_, err := handle.Invoke(invalidCtx, argsJSON)
	require.Error(t, err)
	assert.Equal(t, 0, registrar.expenseCalls)
}

func TestBuildRegisterExpenseToolMissingIdentity(t *testing.T) {
	registrar := &fakeRegistrar{}
	handle := BuildRegisterExpenseTool(registrar)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "Almoço", PaymentMethod: "debit_card"})
	_, err := handle.Invoke(context.Background(), argsJSON)
	require.Error(t, err)
	assert.Equal(t, 0, registrar.expenseCalls)
}

func TestBuildRegisterExpenseToolDelegateError(t *testing.T) {
	registrar := &fakeRegistrar{expenseErr: errors.New("ledger error")}
	handle := BuildRegisterExpenseTool(registrar)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "Almoço", PaymentMethod: "debit_card"})
	_, err := handle.Invoke(identityCtx("wamid1", 0), argsJSON)
	require.Error(t, err)
}

func TestBuildRegisterIncomeToolDelegatesWithoutPaymentMethod(t *testing.T) {
	registrar := &fakeRegistrar{
		incomeResult: usecases.RegisterResult{ResourceID: testResourceID, Kind: "transaction", Outcome: agent.ToolOutcomeRouted},
	}

	handle := BuildRegisterIncomeTool(registrar)
	assert.Equal(t, "register_income", handle.ID())

	argsJSON, _ := json.Marshal(RegisterIncomeInput{AmountCents: 100000, Description: "Salário"})
	out, err := handle.Invoke(identityCtx("wamid2", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterIncomeOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, testResourceID.String(), result.ResourceID)
	assert.Equal(t, "transaction", result.Kind)
	assert.False(t, result.IsReplay)

	assert.Equal(t, 1, registrar.incomeCalls)
	assert.Equal(t, int64(100000), registrar.lastIncome.AmountCents)
	assert.Equal(t, "Salário", registrar.lastIncome.Description)
}

func TestBuildRegisterIncomeToolClarify(t *testing.T) {
	registrar := &fakeRegistrar{incomeResult: usecases.RegisterResult{Outcome: agent.ToolOutcomeClarify}}
	handle := BuildRegisterIncomeTool(registrar)
	argsJSON, _ := json.Marshal(RegisterIncomeInput{AmountCents: 100000, Description: "algo"})
	out, err := handle.Invoke(identityCtx("wamid2", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterIncomeOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome)
	assert.Empty(t, result.ResourceID)
}

func TestBuildRegisterExpenseToolCreditCardDelegatesCardIDAndInstallments(t *testing.T) {
	cardID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	registrar := &fakeRegistrar{
		expenseResult: usecases.RegisterResult{ResourceID: testResourceID, Kind: "transaction", Outcome: agent.ToolOutcomeRouted},
	}

	handle := BuildRegisterExpenseTool(registrar)

	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   30000,
		Description:   "Notebook",
		PaymentMethod: "credit_card",
		CardID:        cardID.String(),
		Installments:  3,
	})
	out, err := handle.Invoke(identityCtx("wamid3", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, testResourceID.String(), result.ResourceID)
	assert.Equal(t, "transaction", result.Kind)

	assert.Equal(t, 1, registrar.expenseCalls)
	assert.Equal(t, "credit_card", registrar.lastExpense.PaymentMethod)
	require.NotNil(t, registrar.lastExpense.CardID)
	assert.Equal(t, cardID, *registrar.lastExpense.CardID)
	assert.Equal(t, 3, registrar.lastExpense.Installments)
	assert.Equal(t, int64(30000), registrar.lastExpense.AmountCents)
}

func TestBuildRegisterExpenseToolCreditCardDefaultsInstallmentsToOne(t *testing.T) {
	cardID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	registrar := &fakeRegistrar{
		expenseResult: usecases.RegisterResult{ResourceID: testResourceID, Kind: "transaction", Outcome: agent.ToolOutcomeRouted},
	}

	handle := BuildRegisterExpenseTool(registrar)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   30000,
		Description:   "Compra à vista",
		PaymentMethod: "credit_card",
		CardID:        cardID.String(),
	})
	_, err := handle.Invoke(identityCtx("wamid3", 0), argsJSON)
	require.NoError(t, err)

	assert.Equal(t, 1, registrar.expenseCalls)
	assert.Equal(t, 1, registrar.lastExpense.Installments)
}

func TestBuildRegisterExpenseToolInvalidCardID(t *testing.T) {
	registrar := &fakeRegistrar{}
	handle := BuildRegisterExpenseTool(registrar)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   30000,
		Description:   "Compra",
		PaymentMethod: "credit_card",
		CardID:        "not-a-uuid",
		Installments:  1,
	})
	_, err := handle.Invoke(identityCtx("wamid3", 0), argsJSON)
	require.Error(t, err)
	assert.Equal(t, 0, registrar.expenseCalls)
}

func TestBuildQueryMonthToolSuccess(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)

	ledger.EXPECT().GetMonthlySummary(mock.Anything, testUserID, "2026-06").
		Return(interfaces.MonthlySummary{
			RefMonth:     "2026-06",
			IncomeCents:  100000,
			OutcomeCents: 60000,
			TotalCents:   40000,
		}, nil).Once()

	ledger.EXPECT().ListMonthlyEntries(mock.Anything, testUserID, "2026-06", "", 50).
		Return([]interfaces.MonthlyEntry{
			{Kind: interfaces.EntryKindTransaction, ID: testResourceID.String(), RefMonth: "2026-06", AmountCents: 5000, Direction: "outcome", Description: "Almoço", CreatedAt: time.Now()},
		}, nil).Once()

	handle := BuildQueryMonthTool(ledger)
	assert.Equal(t, "query_month", handle.ID())

	argsJSON, _ := json.Marshal(QueryMonthInput{
		RefMonth: "2026-06",
	})
	out, err := handle.Invoke(identityCtx("msg-q", 0), argsJSON)
	require.NoError(t, err)

	var result QueryMonthOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "2026-06", result.RefMonth)
	assert.Equal(t, int64(100000), result.IncomeCents)
	assert.Equal(t, int64(60000), result.OutcomeCents)
	assert.Len(t, result.Entries, 1)
}

func TestBuildQueryMonthToolSummaryError(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)

	ledger.EXPECT().GetMonthlySummary(mock.Anything, testUserID, "2026-06").
		Return(interfaces.MonthlySummary{}, errors.New("db error")).Once()

	handle := BuildQueryMonthTool(ledger)
	argsJSON, _ := json.Marshal(QueryMonthInput{RefMonth: "2026-06"})
	_, err := handle.Invoke(identityCtx("msg-q", 0), argsJSON)
	require.Error(t, err)
}

func TestBuildQueryPlanToolSuccess(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)

	planner.EXPECT().GetMonthlySummary(mock.Anything, testUserID, "2026-06").
		Return(interfaces.BudgetSummary{
			Competence:      "2026-06",
			State:           "active",
			AutoDraft:       false,
			TotalSpentCents: 60000,
			Allocations: []interfaces.AllocationSummary{
				{RootSlug: "moradia", SpentCents: 30000},
			},
		}, nil).Once()

	planner.EXPECT().ListAlerts(mock.Anything, testUserID).
		Return([]interfaces.Alert{}, nil).Once()

	handle := BuildQueryPlanTool(planner)
	assert.Equal(t, "query_plan", handle.ID())

	argsJSON, _ := json.Marshal(QueryPlanInput{Competence: "2026-06"})
	out, err := handle.Invoke(identityCtx("msg-q", 0), argsJSON)
	require.NoError(t, err)

	var result QueryPlanOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "2026-06", result.Competence)
	assert.Equal(t, "active", result.State)
	assert.Len(t, result.Allocations, 1)
	assert.Len(t, result.Alerts, 0)
}

func TestBuildAdjustAllocationToolSuccess(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)

	planner.EXPECT().EditCategoryPercentage(mock.Anything, testUserID, "2026-06", "moradia", 30).
		Return(nil).Once()

	handle := BuildAdjustAllocationTool(planner)
	assert.Equal(t, "adjust_allocation", handle.ID())

	argsJSON, _ := json.Marshal(AdjustAllocationInput{
		Competence: "2026-06",
		RootSlug:   "moradia",
		Percentage: 30,
	})
	ctx := agent.WithToolInvocationContext(context.Background(), testUserID.String(), "wamid-adjust", 0)
	out, err := handle.Invoke(ctx, argsJSON)
	require.NoError(t, err)

	var result AdjustAllocationOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.OK)
	assert.Equal(t, "moradia", result.RootSlug)
	assert.Equal(t, 30, result.Percentage)
}

func TestBuildAdjustAllocationToolError(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)

	planner.EXPECT().EditCategoryPercentage(mock.Anything, testUserID, "2026-06", "moradia", 30).
		Return(errors.New("budget error")).Once()

	handle := BuildAdjustAllocationTool(planner)
	argsJSON, _ := json.Marshal(AdjustAllocationInput{
		Competence: "2026-06",
		RootSlug:   "moradia",
		Percentage: 30,
	})
	ctx := agent.WithToolInvocationContext(context.Background(), testUserID.String(), "wamid-adjust", 0)
	_, err := handle.Invoke(ctx, argsJSON)
	require.Error(t, err)
}

func TestBuildClassifyCategoryToolSuccess(t *testing.T) {
	reader := imocks.NewCategoriesReader(t)

	rootID := uuid.New()
	reader.EXPECT().SearchDictionary(mock.Anything, "restaurante", "outcome").
		Return(interfaces.CategorySearchResult{
			Outcome: interfaces.ClassifyOutcomeMatched,
			Version: 1,
			Candidates: []interfaces.CategoryCandidate{
				{CategoryID: testCategoryID, RootCategoryID: rootID, Path: "alimentacao/restaurante", Score: 0.95, IsAmbiguous: false, SignalType: "alias", Confidence: "high", MatchQuality: "exact"},
			},
		}, nil).Once()

	handle := BuildClassifyCategoryTool(reader)
	assert.Equal(t, "classify_category", handle.ID())

	argsJSON, _ := json.Marshal(ClassifyCategoryInput{Term: "restaurante", Kind: "outcome"})
	out, err := handle.Invoke(context.Background(), argsJSON)
	require.NoError(t, err)

	var result ClassifyCategoryOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, rootID.String(), result.CategoryID)
	assert.Equal(t, testCategoryID.String(), result.SubcategoryID)
	assert.Equal(t, "alimentacao/restaurante", result.Path)
	assert.False(t, result.IsAmbiguous)
	assert.Len(t, result.Candidates, 1)
}

func TestBuildClassifyCategoryToolAmbiguous(t *testing.T) {
	reader := imocks.NewCategoriesReader(t)

	reader.EXPECT().SearchDictionary(mock.Anything, "mercado", "outcome").
		Return(interfaces.CategorySearchResult{
			Outcome: interfaces.ClassifyOutcomeAmbiguous,
			Version: 1,
			Candidates: []interfaces.CategoryCandidate{
				{CategoryID: testCategoryID, Path: "alimentacao/mercado", Score: 0.8, IsAmbiguous: false},
				{CategoryID: uuid.New(), Path: "lazer/mercado", Score: 0.75, IsAmbiguous: false},
			},
		}, nil).Once()

	handle := BuildClassifyCategoryTool(reader)
	argsJSON, _ := json.Marshal(ClassifyCategoryInput{Term: "mercado", Kind: "outcome"})
	out, err := handle.Invoke(context.Background(), argsJSON)
	require.NoError(t, err)

	var result ClassifyCategoryOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.IsAmbiguous)
	assert.Len(t, result.Candidates, 2)
}

type fakeEntryEditor struct {
	result usecases.RegisterResult
	err    error
	called bool
	lastID uuid.UUID
}

func (f *fakeEntryEditor) EditEntry(_ context.Context, cmd usecases.EditEntryCommand) (usecases.RegisterResult, error) {
	f.called = true
	f.lastID = cmd.TargetTransactionID
	return f.result, f.err
}

func TestBuildEditEntryTool(t *testing.T) {
	editor := &fakeEntryEditor{result: usecases.RegisterResult{Outcome: agent.ToolOutcomeClarify}}
	handle := BuildEditEntryTool(editor)
	assert.Equal(t, "edit_entry", handle.ID())

	argsJSON, _ := json.Marshal(EditEntryInput{EntryID: testResourceID.String(), AmountCents: 15000})
	out, err := handle.Invoke(identityCtx("wamid-edit-001", 0), argsJSON)
	require.NoError(t, err)

	var result EditEntryOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Equal(t, testResourceID.String(), result.TargetRef)
	assert.True(t, editor.called)
	assert.Equal(t, testResourceID, editor.lastID)
}

func TestBuildDeleteEntryTool(t *testing.T) {
	cardMock := imocks.NewCardManager(t)
	cardMock.EXPECT().
		HasOpenInstallments(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID")).
		Return(false, nil).Maybe()

	engine := &fakeConfirmEngine{
		startResult: wf.RunResult[workflows.ConfirmState]{Status: wf.RunStatusSuspended},
	}
	handle := BuildDeleteEntryTool(engine, fakeConfirmDef(), cardMock)
	assert.Equal(t, "delete_entry", handle.ID())

	argsJSON, _ := json.Marshal(DeleteEntryInput{EntryID: testResourceID.String(), EntryKind: "card_purchase"})
	out, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result DeleteEntryOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Equal(t, testResourceID.String(), result.TargetRef)
	assert.Equal(t, "card_purchase", result.TargetKind)
}

func TestRegisterExpenseOutput_OutcomeField_Routed(t *testing.T) {
	registrar := &fakeRegistrar{
		expenseResult: usecases.RegisterResult{ResourceID: testResourceID, Kind: "transaction", Outcome: agent.ToolOutcomeRouted},
	}

	handle := BuildRegisterExpenseTool(registrar)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 1000, Description: "café", PaymentMethod: "debit_card"})
	out, err := handle.Invoke(identityCtx("wamid1", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "routed", result.Outcome)
	assert.False(t, result.IsReplay)
}

func TestBuildUpdateRecurrenceTool_NeedsConfirmation(t *testing.T) {
	engine := &fakeConfirmEngine{
		startResult: wf.RunResult[workflows.ConfirmState]{Status: wf.RunStatusSuspended},
	}
	handle := BuildUpdateRecurrenceTool(engine, fakeConfirmDef())
	assert.Equal(t, "update_recurrence", handle.ID())
	assert.NotEmpty(t, handle.Description())

	templateID := uuid.New().String()
	argsJSON, _ := json.Marshal(UpdateRecurrenceInput{TemplateID: templateID, Version: 1})
	out, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result UpdateRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Equal(t, templateID, result.TargetRef)
	assert.Equal(t, "recurring_template", result.TargetKind)
}

func TestBuildUpdateRecurrenceTool_AlreadyExists(t *testing.T) {
	engine := &fakeConfirmEngine{
		startErr: wf.ErrRunAlreadyExists,
	}
	handle := BuildUpdateRecurrenceTool(engine, fakeConfirmDef())

	templateID := uuid.New().String()
	argsJSON, _ := json.Marshal(UpdateRecurrenceInput{TemplateID: templateID, Version: 1})
	out, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result UpdateRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Contains(t, result.ImpactNote, "pendente")
}

func TestBuildDeleteRecurrenceTool_NeedsConfirmation(t *testing.T) {
	engine := &fakeConfirmEngine{
		startResult: wf.RunResult[workflows.ConfirmState]{Status: wf.RunStatusSuspended},
	}
	handle := BuildDeleteRecurrenceTool(engine, fakeConfirmDef())
	assert.Equal(t, "delete_recurrence", handle.ID())
	assert.NotEmpty(t, handle.Description())

	templateID := uuid.New().String()
	argsJSON, _ := json.Marshal(DeleteRecurrenceInput{TemplateID: templateID, Version: 2})
	out, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result DeleteRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Equal(t, templateID, result.TargetRef)
	assert.Equal(t, "recurring_template", result.TargetKind)
	assert.Contains(t, result.ImpactNote, "permanentemente")
}

func TestBuildDeleteRecurrenceTool_AlreadyExists(t *testing.T) {
	engine := &fakeConfirmEngine{
		startErr: wf.ErrRunAlreadyExists,
	}
	handle := BuildDeleteRecurrenceTool(engine, fakeConfirmDef())

	argsJSON, _ := json.Marshal(DeleteRecurrenceInput{TemplateID: uuid.New().String(), Version: 1})
	out, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result DeleteRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Contains(t, result.ImpactNote, "pendente")
}

func TestBuildUpdateCardTool_DirectExecution_NoDueDay(t *testing.T) {
	cardMock := imocks.NewCardManager(t)
	nickname := "Nubank"
	cardMock.EXPECT().
		UpdateCard(mock.Anything, mock.AnythingOfType("interfaces.CardUpdate")).
		Return(interfaces.Card{ID: testResourceID.String()}, nil).Once()

	engine := &fakeConfirmEngine{}
	handle := BuildUpdateCardTool(engine, fakeConfirmDef(), cardMock)
	assert.Equal(t, "update_card", handle.ID())
	assert.NotEmpty(t, handle.Description())

	argsJSON, _ := json.Marshal(UpdateCardInput{CardID: testResourceID.String(), Version: 1, Nickname: &nickname})
	out, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result UpdateCardOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.False(t, result.NeedsConfirmation)
	assert.True(t, result.Executed)
}

func TestBuildUpdateCardTool_Gate_WithDueDay(t *testing.T) {
	cardMock := imocks.NewCardManager(t)
	engine := &fakeConfirmEngine{
		startResult: wf.RunResult[workflows.ConfirmState]{Status: wf.RunStatusSuspended},
	}

	handle := BuildUpdateCardTool(engine, fakeConfirmDef(), cardMock)

	dueDay := 15
	argsJSON, _ := json.Marshal(UpdateCardInput{CardID: testResourceID.String(), Version: 1, DueDay: &dueDay})
	out, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result UpdateCardOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.False(t, result.Executed)
	assert.Contains(t, result.ImpactNote, "vencimento")
}

func TestBuildUpdateCardTool_AlreadyExists(t *testing.T) {
	cardMock := imocks.NewCardManager(t)
	engine := &fakeConfirmEngine{
		startErr: wf.ErrRunAlreadyExists,
	}

	handle := BuildUpdateCardTool(engine, fakeConfirmDef(), cardMock)

	dueDay := 20
	argsJSON, _ := json.Marshal(UpdateCardInput{CardID: testResourceID.String(), Version: 1, DueDay: &dueDay})
	out, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result UpdateCardOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Contains(t, result.ImpactNote, "pendente")
}

func TestRegisterIncomeOutput_OutcomeField_Replay(t *testing.T) {
	registrar := &fakeRegistrar{
		incomeResult: usecases.RegisterResult{ResourceID: testResourceID, Kind: "transaction", Outcome: agent.ToolOutcomeReplay},
	}

	handle := BuildRegisterIncomeTool(registrar)
	argsJSON, _ := json.Marshal(RegisterIncomeInput{AmountCents: 2000, Description: "salário"})
	out, err := handle.Invoke(identityCtx("wamid2", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterIncomeOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "replay", result.Outcome)
	assert.True(t, result.IsReplay)
}

func TestRegisterExpenseOutput_OutcomeField_Reconciled(t *testing.T) {
	registrar := &fakeRegistrar{
		expenseResult: usecases.RegisterResult{ResourceID: testResourceID, Kind: "transaction", Outcome: agent.ToolOutcomeReconciled},
	}

	handle := BuildRegisterExpenseTool(registrar)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "café", PaymentMethod: "pix"})
	out, err := handle.Invoke(identityCtx("wamid-rec", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "reconciled", result.Outcome)
	assert.False(t, result.IsReplay)
	assert.Equal(t, testResourceID.String(), result.ResourceID)
}

func TestBuildResolveCardToolFound(t *testing.T) {
	cardMgr := imocks.NewCardManager(t)
	cardID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	cardMgr.EXPECT().ResolveCardByNickname(mock.Anything, testUserID, "Nubank").
		Return(interfaces.Card{ID: cardID.String(), Nickname: "Nubank", Bank: "Nubank", DueDay: 10}, nil).Once()

	handle := BuildResolveCardTool(cardMgr)
	assert.Equal(t, "resolve_card", handle.ID())

	argsJSON, _ := json.Marshal(ResolveCardInput{Nickname: "Nubank"})
	out, err := handle.Invoke(identityCtx("wamid-rc", 0), argsJSON)
	require.NoError(t, err)

	var result ResolveCardOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.Found)
	assert.Equal(t, cardID.String(), result.CardID)
	assert.Equal(t, 10, result.DueDay)
}

func TestBuildResolveCardToolNotFound(t *testing.T) {
	cardMgr := imocks.NewCardManager(t)
	cardMgr.EXPECT().ResolveCardByNickname(mock.Anything, testUserID, "Inexistente").
		Return(interfaces.Card{}, interfaces.ErrCardNotFound).Once()

	handle := BuildResolveCardTool(cardMgr)
	argsJSON, _ := json.Marshal(ResolveCardInput{Nickname: "Inexistente"})
	out, err := handle.Invoke(identityCtx("wamid-rc", 0), argsJSON)
	require.NoError(t, err)

	var result ResolveCardOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.False(t, result.Found)
	assert.Empty(t, result.CardID)
}

func TestBuildRegisterExpenseToolCeilingRejectsWithoutRegistrarCall(t *testing.T) {
	registrar := &fakeRegistrar{}

	handle := BuildRegisterExpenseTool(registrar)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   maxEntryAmountCents + 1,
		Description:   "compra absurda",
		PaymentMethod: "pix",
	})
	out, err := handle.Invoke(identityCtx("wamid-ceiling", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome, "8.2: teto deve retornar clarify sem chamar o registrar")
	assert.NotEmpty(t, result.Message, "8.2: mensagem de teto deve ser não-vazia")
	assert.Equal(t, 0, registrar.expenseCalls, "8.2: registrar NÃO deve ser chamado quando acima do teto")
}

func TestBuildRegisterIncomeToolCeilingRejectsWithoutRegistrarCall(t *testing.T) {
	registrar := &fakeRegistrar{}

	handle := BuildRegisterIncomeTool(registrar)
	argsJSON, _ := json.Marshal(RegisterIncomeInput{
		AmountCents: maxEntryAmountCents + 1,
		Description: "receita absurda",
	})
	out, err := handle.Invoke(identityCtx("wamid-income-ceiling", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterIncomeOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome, "8.2: teto income deve retornar clarify sem chamar o registrar")
	assert.NotEmpty(t, result.Message, "8.2: mensagem de teto income deve ser não-vazia")
	assert.Equal(t, 0, registrar.incomeCalls, "8.2: registrar NÃO deve ser chamado quando acima do teto (income)")
}
