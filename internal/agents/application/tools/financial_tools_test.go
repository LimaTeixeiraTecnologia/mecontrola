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

type fakeDestructiveManageEngine struct {
	startResult wf.RunResult[workflows.DestructiveManageState]
	startErr    error
}

func (f *fakeDestructiveManageEngine) Start(_ context.Context, _ wf.Definition[workflows.DestructiveManageState], _ string, _ workflows.DestructiveManageState) (wf.RunResult[workflows.DestructiveManageState], error) {
	return f.startResult, f.startErr
}

func (f *fakeDestructiveManageEngine) Resume(_ context.Context, _ wf.Definition[workflows.DestructiveManageState], _ string, _ []byte) (wf.RunResult[workflows.DestructiveManageState], error) {
	return wf.RunResult[workflows.DestructiveManageState]{}, nil
}

func (f *fakeDestructiveManageEngine) LoadLatestState(_ context.Context, _ wf.Definition[workflows.DestructiveManageState], _ string) (workflows.DestructiveManageState, wf.Snapshot, bool, error) {
	return workflows.DestructiveManageState{}, wf.Snapshot{}, false, nil
}

func fakeDestructiveManageDef() wf.Definition[workflows.DestructiveManageState] {
	return wf.Definition[workflows.DestructiveManageState]{
		ID:      workflows.DestructiveManageWorkflowID,
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

	handle := BuildRegisterExpenseTool(registrar, imocks.NewCardManager(t))
	assert.Equal(t, "register_expense", handle.ID())
	assert.NotEmpty(t, handle.Description())

	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   5000,
		Description:   "Almoço",
		PaymentMethod: "debit_card",
	})
	out, verbatimText, err := handle.Invoke(identityCtx("wamid1", 2), argsJSON)
	require.NoError(t, err)
	assert.Empty(t, verbatimText)

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

	handle := BuildRegisterExpenseTool(registrar, imocks.NewCardManager(t))
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "Almoço", PaymentMethod: "debit_card"})
	out, _, err := handle.Invoke(identityCtx("wamid1", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.IsReplay)
	assert.Equal(t, "replay", result.Outcome)
}

func TestBuildRegisterExpenseToolClarifyOmitsResource(t *testing.T) {
	registrar := &fakeRegistrar{
		expenseResult: usecases.RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: "Percebi mais de um lançamento na mesma mensagem."},
	}

	handle := BuildRegisterExpenseTool(registrar, imocks.NewCardManager(t))
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "algo ambíguo", PaymentMethod: "pix"})
	out, verbatimText, err := handle.Invoke(identityCtx("wamid1", 0), argsJSON)
	require.NoError(t, err)
	assert.Equal(t, "Percebi mais de um lançamento na mesma mensagem.", verbatimText)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome)
	assert.Empty(t, result.ResourceID)
	assert.False(t, result.IsReplay)
}

func TestBuildRegisterExpenseToolInvalidUserID(t *testing.T) {
	registrar := &fakeRegistrar{}
	handle := BuildRegisterExpenseTool(registrar, imocks.NewCardManager(t))
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "Almoço", PaymentMethod: "debit_card"})
	invalidCtx := agent.WithToolInvocationContext(context.Background(), "not-a-uuid", "wamid1", 0)
	_, _, err := handle.Invoke(invalidCtx, argsJSON)
	require.Error(t, err)
	assert.Equal(t, 0, registrar.expenseCalls)
}

func TestBuildRegisterExpenseToolMissingIdentity(t *testing.T) {
	registrar := &fakeRegistrar{}
	handle := BuildRegisterExpenseTool(registrar, imocks.NewCardManager(t))
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "Almoço", PaymentMethod: "debit_card"})
	_, _, err := handle.Invoke(context.Background(), argsJSON)
	require.Error(t, err)
	assert.Equal(t, 0, registrar.expenseCalls)
}

func TestBuildRegisterExpenseToolDelegateError(t *testing.T) {
	registrar := &fakeRegistrar{expenseErr: errors.New("ledger error")}
	handle := BuildRegisterExpenseTool(registrar, imocks.NewCardManager(t))
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "Almoço", PaymentMethod: "debit_card"})
	_, _, err := handle.Invoke(identityCtx("wamid1", 0), argsJSON)
	require.Error(t, err)
}

func TestBuildRegisterIncomeToolDelegatesWithoutPaymentMethod(t *testing.T) {
	registrar := &fakeRegistrar{
		incomeResult: usecases.RegisterResult{ResourceID: testResourceID, Kind: "transaction", Outcome: agent.ToolOutcomeRouted},
	}

	handle := BuildRegisterIncomeTool(registrar)
	assert.Equal(t, "register_income", handle.ID())

	argsJSON, _ := json.Marshal(RegisterIncomeInput{AmountCents: 100000, Description: "Salário"})
	out, _, err := handle.Invoke(identityCtx("wamid2", 0), argsJSON)
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
	registrar := &fakeRegistrar{incomeResult: usecases.RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: "Percebi mais de um lançamento na mesma mensagem."}}
	handle := BuildRegisterIncomeTool(registrar)
	argsJSON, _ := json.Marshal(RegisterIncomeInput{AmountCents: 100000, Description: "algo"})
	out, verbatimText, err := handle.Invoke(identityCtx("wamid2", 0), argsJSON)
	require.NoError(t, err)
	assert.Equal(t, "Percebi mais de um lançamento na mesma mensagem.", verbatimText)

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

	cards := imocks.NewCardManager(t)
	cards.EXPECT().GetCard(mock.Anything, cardID, testUserID).Return(interfaces.Card{ID: cardID.String()}, nil).Once()
	handle := BuildRegisterExpenseTool(registrar, cards)

	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   30000,
		Description:   "Notebook",
		PaymentMethod: "credit_card",
		CardID:        cardID.String(),
		Installments:  3,
	})
	out, _, err := handle.Invoke(identityCtx("wamid3", 0), argsJSON)
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

	cards := imocks.NewCardManager(t)
	cards.EXPECT().GetCard(mock.Anything, cardID, testUserID).Return(interfaces.Card{ID: cardID.String()}, nil).Once()
	handle := BuildRegisterExpenseTool(registrar, cards)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   30000,
		Description:   "Compra à vista",
		PaymentMethod: "credit_card",
		CardID:        cardID.String(),
	})
	_, _, err := handle.Invoke(identityCtx("wamid3", 0), argsJSON)
	require.NoError(t, err)

	assert.Equal(t, 1, registrar.expenseCalls)
	assert.Equal(t, 1, registrar.lastExpense.Installments)
}

func TestBuildRegisterExpenseToolInvalidCardID(t *testing.T) {
	registrar := &fakeRegistrar{}
	handle := BuildRegisterExpenseTool(registrar, imocks.NewCardManager(t))
	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   30000,
		Description:   "Compra",
		PaymentMethod: "credit_card",
		CardID:        "not-a-uuid",
		Installments:  1,
	})
	_, _, err := handle.Invoke(identityCtx("wamid3", 0), argsJSON)
	require.Error(t, err)
	assert.Equal(t, 0, registrar.expenseCalls)
}

func TestBuildRegisterExpenseToolCardNotFoundAsksClarify(t *testing.T) {
	cardID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	registrar := &fakeRegistrar{}

	cards := imocks.NewCardManager(t)
	cards.EXPECT().GetCard(mock.Anything, cardID, testUserID).Return(interfaces.Card{}, interfaces.ErrCardNotFound).Once()
	handle := BuildRegisterExpenseTool(registrar, cards)

	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   30000,
		Description:   "Notebook",
		PaymentMethod: "credit_card",
		CardID:        cardID.String(),
		Installments:  1,
	})
	out, verbatimText, err := handle.Invoke(identityCtx("wamid3", 0), argsJSON)
	require.NoError(t, err)
	assert.NotEmpty(t, verbatimText)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome)
	assert.Empty(t, result.ResourceID)
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
		MonthRefKind: "explicit",
		Year:         2026,
		Month:        6,
	})
	out, _, err := handle.Invoke(identityCtx("msg-q", 0), argsJSON)
	require.NoError(t, err)

	var result QueryMonthOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "ok", result.Outcome)
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
	argsJSON, _ := json.Marshal(QueryMonthInput{MonthRefKind: "explicit", Year: 2026, Month: 6})
	_, _, err := handle.Invoke(identityCtx("msg-q", 0), argsJSON)
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

	argsJSON, _ := json.Marshal(QueryPlanInput{MonthRefKind: "explicit", Year: 2026, Month: 6})
	out, _, err := handle.Invoke(identityCtx("msg-q", 0), argsJSON)
	require.NoError(t, err)

	var result QueryPlanOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "ok", result.Outcome)
	assert.Equal(t, "2026-06", result.Competence)
	assert.Equal(t, "active", result.State)
	assert.Len(t, result.Allocations, 1)
	assert.Len(t, result.Alerts, 0)
}

func TestBuildQueryMonthToolCurrentFallback(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	expected := time.Now().In(loc).Format("2006-01")

	ledger.EXPECT().GetMonthlySummary(mock.Anything, testUserID, expected).
		Return(interfaces.MonthlySummary{RefMonth: expected}, nil).Once()
	ledger.EXPECT().ListMonthlyEntries(mock.Anything, testUserID, expected, "", 50).
		Return([]interfaces.MonthlyEntry{}, nil).Once()

	handle := BuildQueryMonthTool(ledger)
	argsJSON, _ := json.Marshal(QueryMonthInput{})
	out, _, invokeErr := handle.Invoke(identityCtx("msg-q-fallback", 0), argsJSON)
	require.NoError(t, invokeErr)

	var result QueryMonthOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "ok", result.Outcome)
	assert.Equal(t, expected, result.RefMonth)
}

func TestBuildQueryMonthToolPreviousResolvesToPriorMonth(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	now := time.Now().In(loc)
	prev := now.AddDate(0, -1, 0).Format("2006-01")

	ledger.EXPECT().GetMonthlySummary(mock.Anything, testUserID, prev).
		Return(interfaces.MonthlySummary{RefMonth: prev}, nil).Once()
	ledger.EXPECT().ListMonthlyEntries(mock.Anything, testUserID, prev, "", 50).
		Return([]interfaces.MonthlyEntry{}, nil).Once()

	handle := BuildQueryMonthTool(ledger)
	argsJSON, _ := json.Marshal(QueryMonthInput{MonthRefKind: "previous"})
	out, _, invokeErr := handle.Invoke(identityCtx("msg-q-prev", 0), argsJSON)
	require.NoError(t, invokeErr)

	var result QueryMonthOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "ok", result.Outcome)
	assert.Equal(t, prev, result.RefMonth)
}

func TestBuildQueryMonthToolNextResolvesToNextMonth(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	now := time.Now().In(loc)
	next := now.AddDate(0, 1, 0).Format("2006-01")

	ledger.EXPECT().GetMonthlySummary(mock.Anything, testUserID, next).
		Return(interfaces.MonthlySummary{RefMonth: next}, nil).Once()
	ledger.EXPECT().ListMonthlyEntries(mock.Anything, testUserID, next, "", 50).
		Return([]interfaces.MonthlyEntry{}, nil).Once()

	handle := BuildQueryMonthTool(ledger)
	argsJSON, _ := json.Marshal(QueryMonthInput{MonthRefKind: "next"})
	out, _, invokeErr := handle.Invoke(identityCtx("msg-q-next", 0), argsJSON)
	require.NoError(t, invokeErr)

	var result QueryMonthOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "ok", result.Outcome)
	assert.Equal(t, next, result.RefMonth)
}

func TestBuildQueryMonthToolNamedWithoutYearReturnsClarify(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)

	handle := BuildQueryMonthTool(ledger)
	argsJSON, _ := json.Marshal(QueryMonthInput{MonthRefKind: "named_without_year"})
	out, _, err := handle.Invoke(identityCtx("msg-q-clarify-year", 0), argsJSON)
	require.NoError(t, err)

	var result QueryMonthOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome)
	assert.NotEmpty(t, result.ClarifyPrompt)
}

func TestBuildQueryMonthToolUnknownReturnsClarify(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)

	handle := BuildQueryMonthTool(ledger)
	argsJSON, _ := json.Marshal(QueryMonthInput{MonthRefKind: "unknown"})
	out, _, err := handle.Invoke(identityCtx("msg-q-clarify-unknown", 0), argsJSON)
	require.NoError(t, err)

	var result QueryMonthOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome)
	assert.NotEmpty(t, result.ClarifyPrompt)
}

func TestBuildQueryMonthToolInvalidMonthRefKindErrors(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)

	handle := BuildQueryMonthTool(ledger)
	argsJSON, _ := json.Marshal(QueryMonthInput{MonthRefKind: "nao-existe"})
	_, _, err := handle.Invoke(identityCtx("msg-q-invalid", 0), argsJSON)
	require.Error(t, err)
}

func TestBuildQueryPlanToolCurrentFallback(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)
	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	expected := time.Now().In(loc).Format("2006-01")

	planner.EXPECT().GetMonthlySummary(mock.Anything, testUserID, expected).
		Return(interfaces.BudgetSummary{Competence: expected, State: "active"}, nil).Once()
	planner.EXPECT().ListAlerts(mock.Anything, testUserID).
		Return([]interfaces.Alert{}, nil).Once()

	handle := BuildQueryPlanTool(planner)
	argsJSON, _ := json.Marshal(QueryPlanInput{})
	out, _, invokeErr := handle.Invoke(identityCtx("msg-p-fallback", 0), argsJSON)
	require.NoError(t, invokeErr)

	var result QueryPlanOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "ok", result.Outcome)
	assert.Equal(t, expected, result.Competence)
}

func TestBuildQueryPlanToolNotFoundReturnsCleanOutcome(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)

	planner.EXPECT().GetMonthlySummary(mock.Anything, testUserID, "2026-06").
		Return(interfaces.BudgetSummary{}, interfaces.ErrBudgetNotFound).Once()

	handle := BuildQueryPlanTool(planner)
	argsJSON, _ := json.Marshal(QueryPlanInput{MonthRefKind: "explicit", Year: 2026, Month: 6})
	out, _, err := handle.Invoke(identityCtx("msg-p-notfound", 0), argsJSON)
	require.NoError(t, err)

	var result QueryPlanOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "not_found", result.Outcome)
	assert.Equal(t, "2026-06", result.Competence)
	assert.Empty(t, result.Allocations)
}

func TestBuildQueryPlanToolNamedWithoutYearReturnsClarify(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)

	handle := BuildQueryPlanTool(planner)
	argsJSON, _ := json.Marshal(QueryPlanInput{MonthRefKind: "named_without_year"})
	out, _, err := handle.Invoke(identityCtx("msg-p-clarify-year", 0), argsJSON)
	require.NoError(t, err)

	var result QueryPlanOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome)
	assert.NotEmpty(t, result.ClarifyPrompt)
}

func TestBuildQueryPlanToolUnknownReturnsClarify(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)

	handle := BuildQueryPlanTool(planner)
	argsJSON, _ := json.Marshal(QueryPlanInput{MonthRefKind: "unknown"})
	out, _, err := handle.Invoke(identityCtx("msg-p-clarify-unknown", 0), argsJSON)
	require.NoError(t, err)

	var result QueryPlanOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome)
	assert.NotEmpty(t, result.ClarifyPrompt)
}

func TestBuildQueryPlanToolInvalidMonthRefKindErrors(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)

	handle := BuildQueryPlanTool(planner)
	argsJSON, _ := json.Marshal(QueryPlanInput{MonthRefKind: "nao-existe"})
	_, _, err := handle.Invoke(identityCtx("msg-p-invalid", 0), argsJSON)
	require.Error(t, err)
}

func TestBuildAdjustAllocationToolSuccess(t *testing.T) {
	engine := newFakeBudgetManageEngine()
	handle := BuildAdjustAllocationTool(engine, fakeBudgetManageDef())
	assert.Equal(t, "adjust_allocation", handle.ID())

	argsJSON, _ := json.Marshal(AdjustAllocationInput{MonthRefKind: "current"})
	out, _, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result AdjustAllocationOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "started", result.Outcome)
	assert.True(t, engine.startCalled)
	assert.Equal(t, workflows.BudgetManageOpEditDistribution, engine.lastState.Operation)
}

func TestBuildAdjustAllocationToolAlreadyExists(t *testing.T) {
	engine := &fakeBudgetManageEngine{startErr: wf.ErrRunAlreadyExists}
	handle := BuildAdjustAllocationTool(engine, fakeBudgetManageDef())

	argsJSON, _ := json.Marshal(AdjustAllocationInput{MonthRefKind: "current"})
	out, _, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result AdjustAllocationOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "pending_exists", result.Outcome)
	assert.NotEmpty(t, result.ClarifyPrompt)
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
	out, _, err := handle.Invoke(context.Background(), argsJSON)
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
	out, _, err := handle.Invoke(context.Background(), argsJSON)
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
	out, verbatimText, err := handle.Invoke(identityCtx("wamid-edit-001", 0), argsJSON)
	require.NoError(t, err)

	var result EditEntryOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Equal(t, testResourceID.String(), result.TargetRef)
	assert.True(t, editor.called)
	assert.Equal(t, testResourceID, editor.lastID)
	assert.NotEmpty(t, verbatimText)
	assert.Equal(t, result.ImpactNote, verbatimText)
}

func TestBuildDeleteEntryTool(t *testing.T) {
	cardMock := imocks.NewCardManager(t)
	cardMock.EXPECT().
		HasOpenInstallments(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("uuid.UUID")).
		Return(false, nil).Maybe()

	engine := &fakeDestructiveManageEngine{
		startResult: wf.RunResult[workflows.DestructiveManageState]{Status: wf.RunStatusSuspended},
	}
	handle := BuildDeleteEntryTool(engine, fakeDestructiveManageDef(), cardMock)
	assert.Equal(t, "delete_entry", handle.ID())

	argsJSON, _ := json.Marshal(DeleteEntryInput{EntryID: testResourceID.String(), EntryKind: "card_purchase"})
	out, verbatimText, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result DeleteEntryOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Equal(t, testResourceID.String(), result.TargetRef)
	assert.Equal(t, "card_purchase", result.TargetKind)
	assert.NotEmpty(t, verbatimText)
	assert.Equal(t, result.ImpactNote, verbatimText)
}

func TestRegisterExpenseOutput_OutcomeField_Routed(t *testing.T) {
	registrar := &fakeRegistrar{
		expenseResult: usecases.RegisterResult{ResourceID: testResourceID, Kind: "transaction", Outcome: agent.ToolOutcomeRouted},
	}

	handle := BuildRegisterExpenseTool(registrar, imocks.NewCardManager(t))
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 1000, Description: "café", PaymentMethod: "debit_card"})
	out, _, err := handle.Invoke(identityCtx("wamid1", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "routed", result.Outcome)
	assert.False(t, result.IsReplay)
}

func TestBuildUpdateRecurrenceTool_NeedsConfirmation(t *testing.T) {
	engine := &fakeDestructiveManageEngine{
		startResult: wf.RunResult[workflows.DestructiveManageState]{Status: wf.RunStatusSuspended},
	}
	handle := BuildUpdateRecurrenceTool(engine, fakeDestructiveManageDef())
	assert.Equal(t, "update_recurrence", handle.ID())
	assert.NotEmpty(t, handle.Description())

	templateID := uuid.New().String()
	argsJSON, _ := json.Marshal(UpdateRecurrenceInput{TemplateID: templateID, Version: 1})
	out, _, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result UpdateRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Equal(t, templateID, result.TargetRef)
	assert.Equal(t, "recurring_template", result.TargetKind)
}

func TestBuildUpdateRecurrenceTool_AlreadyExists(t *testing.T) {
	engine := &fakeDestructiveManageEngine{
		startErr: wf.ErrRunAlreadyExists,
	}
	handle := BuildUpdateRecurrenceTool(engine, fakeDestructiveManageDef())

	templateID := uuid.New().String()
	argsJSON, _ := json.Marshal(UpdateRecurrenceInput{TemplateID: templateID, Version: 1})
	out, _, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result UpdateRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Contains(t, result.ImpactNote, "pendente")
}

func TestBuildDeleteRecurrenceTool_NeedsConfirmation(t *testing.T) {
	engine := &fakeDestructiveManageEngine{
		startResult: wf.RunResult[workflows.DestructiveManageState]{Status: wf.RunStatusSuspended},
	}
	handle := BuildDeleteRecurrenceTool(engine, fakeDestructiveManageDef())
	assert.Equal(t, "delete_recurrence", handle.ID())
	assert.NotEmpty(t, handle.Description())

	templateID := uuid.New().String()
	argsJSON, _ := json.Marshal(DeleteRecurrenceInput{TemplateID: templateID, Version: 2})
	out, _, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result DeleteRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Equal(t, templateID, result.TargetRef)
	assert.Equal(t, "recurring_template", result.TargetKind)
	assert.Contains(t, result.ImpactNote, "permanentemente")
}

func TestBuildDeleteRecurrenceTool_AlreadyExists(t *testing.T) {
	engine := &fakeDestructiveManageEngine{
		startErr: wf.ErrRunAlreadyExists,
	}
	handle := BuildDeleteRecurrenceTool(engine, fakeDestructiveManageDef())

	argsJSON, _ := json.Marshal(DeleteRecurrenceInput{TemplateID: uuid.New().String(), Version: 1})
	out, _, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result DeleteRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Contains(t, result.ImpactNote, "pendente")
}

func TestBuildUpdateCardTool_NicknameOnlyNeedsConfirmation(t *testing.T) {
	nickname := "Nubank"
	engine := &fakeCardManageEngine{
		startResult: wf.RunResult[workflows.CardManageState]{
			State: workflows.CardManageState{ResponseText: "⚠️ Confirma a atualização?"},
		},
	}
	handle := BuildUpdateCardTool(engine, fakeCardManageDef())
	assert.Equal(t, "update_card", handle.ID())
	assert.NotEmpty(t, handle.Description())

	argsJSON, _ := json.Marshal(UpdateCardInput{CardID: testResourceID.String(), Version: 1, Nickname: &nickname})
	out, _, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result UpdateCardOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, updateCardOutcomeNeedsConfirmation, result.Outcome)
	assert.NotEmpty(t, result.ConfirmationPrompt)
	assert.True(t, engine.startCalled)
	assert.True(t, engine.lastState.NicknameProvided)
	assert.False(t, engine.lastState.DueDayProvided)
}

func TestBuildUpdateCardTool_Gate_WithDueDay(t *testing.T) {
	engine := &fakeCardManageEngine{
		startResult: wf.RunResult[workflows.CardManageState]{
			State: workflows.CardManageState{ResponseText: "⚠️ Confirma a atualização do vencimento?"},
		},
	}

	handle := BuildUpdateCardTool(engine, fakeCardManageDef())

	dueDay := 15
	argsJSON, _ := json.Marshal(UpdateCardInput{CardID: testResourceID.String(), Version: 1, DueDay: &dueDay})
	out, _, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result UpdateCardOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, updateCardOutcomeNeedsConfirmation, result.Outcome)
	assert.Contains(t, result.ConfirmationPrompt, "vencimento")
	assert.True(t, engine.lastState.DueDayProvided)
	assert.Equal(t, dueDay, engine.lastState.DueDay)
}

func TestBuildUpdateCardTool_AlreadyExists(t *testing.T) {
	engine := &fakeCardManageEngine{
		startErr: wf.ErrRunAlreadyExists,
	}

	handle := BuildUpdateCardTool(engine, fakeCardManageDef())

	dueDay := 20
	argsJSON, _ := json.Marshal(UpdateCardInput{CardID: testResourceID.String(), Version: 1, DueDay: &dueDay})
	out, _, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result UpdateCardOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, updateCardOutcomePendingConfirmationExists, result.Outcome)
	assert.Contains(t, result.ConfirmationPrompt, "pendente")
}

func TestRegisterIncomeOutput_OutcomeField_Replay(t *testing.T) {
	registrar := &fakeRegistrar{
		incomeResult: usecases.RegisterResult{ResourceID: testResourceID, Kind: "transaction", Outcome: agent.ToolOutcomeReplay},
	}

	handle := BuildRegisterIncomeTool(registrar)
	argsJSON, _ := json.Marshal(RegisterIncomeInput{AmountCents: 2000, Description: "salário"})
	out, _, err := handle.Invoke(identityCtx("wamid2", 0), argsJSON)
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

	handle := BuildRegisterExpenseTool(registrar, imocks.NewCardManager(t))
	argsJSON, _ := json.Marshal(RegisterExpenseInput{AmountCents: 5000, Description: "café", PaymentMethod: "pix"})
	out, _, err := handle.Invoke(identityCtx("wamid-rec", 0), argsJSON)
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
	out, _, err := handle.Invoke(identityCtx("wamid-rc", 0), argsJSON)
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
	out, _, err := handle.Invoke(identityCtx("wamid-rc", 0), argsJSON)
	require.NoError(t, err)

	var result ResolveCardOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.False(t, result.Found)
	assert.Empty(t, result.CardID)
}

func TestBuildRegisterExpenseToolCeilingRejectsWithoutRegistrarCall(t *testing.T) {
	registrar := &fakeRegistrar{}

	handle := BuildRegisterExpenseTool(registrar, imocks.NewCardManager(t))
	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   maxEntryAmountCents + 1,
		Description:   "compra absurda",
		PaymentMethod: "pix",
	})
	out, _, err := handle.Invoke(identityCtx("wamid-ceiling", 0), argsJSON)
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
	out, _, err := handle.Invoke(identityCtx("wamid-income-ceiling", 0), argsJSON)
	require.NoError(t, err)

	var result RegisterIncomeOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome, "8.2: teto income deve retornar clarify sem chamar o registrar")
	assert.NotEmpty(t, result.Message, "8.2: mensagem de teto income deve ser não-vazia")
	assert.Equal(t, 0, registrar.incomeCalls, "8.2: registrar NÃO deve ser chamado quando acima do teto (income)")
}
