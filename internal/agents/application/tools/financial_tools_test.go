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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/tools/mocks"
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

var testUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
var testResourceID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
var testCategoryID = uuid.MustParse("00000000-0000-0000-0000-000000000003")

func TestBuildRegisterExpenseToolSuccess(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	writer := mocks.NewIdempotentWriter(t)

	ledger.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
		Return(interfaces.EntryRef{ID: testResourceID, Kind: "transaction"}, nil).Once()

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid1", 0, "create_expense", "transaction", mock.AnythingOfType("usecases.WriteFn")).
		RunAndReturn(func(ctx context.Context, userID uuid.UUID, wamid string, itemSeq int, operation, resourceKind string, write usecases.WriteFn) (usecases.IdempotentWriteResult, error) {
			id, _, err := write(ctx)
			if err != nil {
				return usecases.IdempotentWriteResult{}, err
			}
			return usecases.IdempotentWriteResult{ResourceID: id, Outcome: agent.ToolOutcomeRouted}, nil
		}).Once()

	handle := BuildRegisterExpenseTool(ledger, writer)
	assert.Equal(t, "register_expense", handle.ID())
	assert.NotEmpty(t, handle.Description())

	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		Wamid:         "wamid1",
		UserID:        testUserID.String(),
		AmountCents:   5000,
		Description:   "Almoço",
		PaymentMethod: "debit",
	})
	out, err := handle.Invoke(context.Background(), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, testResourceID.String(), result.ResourceID)
	assert.Equal(t, "transaction", result.Kind)
	assert.False(t, result.IsReplay)
}

func TestBuildRegisterExpenseToolReplay(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	writer := mocks.NewIdempotentWriter(t)

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid1", 0, "create_expense", "transaction", mock.AnythingOfType("usecases.WriteFn")).
		Return(usecases.IdempotentWriteResult{ResourceID: testResourceID, Outcome: agent.ToolOutcomeReplay}, nil).Once()

	handle := BuildRegisterExpenseTool(ledger, writer)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		Wamid:         "wamid1",
		UserID:        testUserID.String(),
		AmountCents:   5000,
		Description:   "Almoço",
		PaymentMethod: "debit",
	})
	out, err := handle.Invoke(context.Background(), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.IsReplay)
}

func TestBuildRegisterExpenseToolInvalidUserID(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	writer := mocks.NewIdempotentWriter(t)

	handle := BuildRegisterExpenseTool(ledger, writer)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		Wamid:         "wamid1",
		UserID:        "not-a-uuid",
		AmountCents:   5000,
		Description:   "Almoço",
		PaymentMethod: "debit",
	})
	_, err := handle.Invoke(context.Background(), argsJSON)
	require.Error(t, err)
}

func TestBuildRegisterExpenseToolWriterError(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	writer := mocks.NewIdempotentWriter(t)

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid1", 0, "create_expense", "transaction", mock.AnythingOfType("usecases.WriteFn")).
		Return(usecases.IdempotentWriteResult{}, errors.New("ledger error")).Once()

	handle := BuildRegisterExpenseTool(ledger, writer)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		Wamid:         "wamid1",
		UserID:        testUserID.String(),
		AmountCents:   5000,
		Description:   "Almoço",
		PaymentMethod: "debit",
	})
	_, err := handle.Invoke(context.Background(), argsJSON)
	require.Error(t, err)
}

func TestBuildRegisterExpenseToolDefaultsOccurredAtToToday(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	writer := mocks.NewIdempotentWriter(t)

	var captured interfaces.RawTransaction
	ledger.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
		RunAndReturn(func(_ context.Context, raw interfaces.RawTransaction) (interfaces.EntryRef, error) {
			captured = raw
			return interfaces.EntryRef{ID: testResourceID, Kind: "transaction"}, nil
		}).Once()

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid-default", 0, "create_expense", "transaction", mock.AnythingOfType("usecases.WriteFn")).
		RunAndReturn(func(ctx context.Context, userID uuid.UUID, wamid string, itemSeq int, operation, resourceKind string, write usecases.WriteFn) (usecases.IdempotentWriteResult, error) {
			id, _, err := write(ctx)
			if err != nil {
				return usecases.IdempotentWriteResult{}, err
			}
			return usecases.IdempotentWriteResult{ResourceID: id, Outcome: agent.ToolOutcomeRouted}, nil
		}).Once()

	handle := BuildRegisterExpenseTool(ledger, writer)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		Wamid:         "wamid-default",
		UserID:        testUserID.String(),
		AmountCents:   5000,
		Description:   "Almoço sem data",
		PaymentMethod: "debit",
	})
	_, err := handle.Invoke(context.Background(), argsJSON)
	require.NoError(t, err)

	loc, locErr := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, locErr)
	today := time.Now().In(loc).Format("2006-01-02")
	assert.Equal(t, today, captured.OccurredAt)
}

func TestBuildRegisterExpenseToolMultipleItemSeq(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	writer := mocks.NewIdempotentWriter(t)

	ledger.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
		Return(interfaces.EntryRef{ID: testResourceID, Kind: "transaction"}, nil).Twice()

	writeReturn := func(ctx context.Context, userID uuid.UUID, wamid string, itemSeq int, operation, resourceKind string, write usecases.WriteFn) (usecases.IdempotentWriteResult, error) {
		id, _, err := write(ctx)
		if err != nil {
			return usecases.IdempotentWriteResult{}, err
		}
		return usecases.IdempotentWriteResult{ResourceID: id, Outcome: agent.ToolOutcomeRouted}, nil
	}

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid-multi", 0, "create_expense", "transaction", mock.AnythingOfType("usecases.WriteFn")).
		RunAndReturn(writeReturn).Once()
	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid-multi", 1, "create_expense", "transaction", mock.AnythingOfType("usecases.WriteFn")).
		RunAndReturn(writeReturn).Once()

	handle := BuildRegisterExpenseTool(ledger, writer)

	first, _ := json.Marshal(RegisterExpenseInput{
		Wamid: "wamid-multi", ItemSeq: 0, UserID: testUserID.String(),
		AmountCents: 5000, Description: "Item 0", PaymentMethod: "debit",
	})
	_, err := handle.Invoke(context.Background(), first)
	require.NoError(t, err)

	second, _ := json.Marshal(RegisterExpenseInput{
		Wamid: "wamid-multi", ItemSeq: 1, UserID: testUserID.String(),
		AmountCents: 7000, Description: "Item 1", PaymentMethod: "debit",
	})
	_, err = handle.Invoke(context.Background(), second)
	require.NoError(t, err)
}

func TestBuildRegisterIncomeToolSuccess(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	writer := mocks.NewIdempotentWriter(t)

	ledger.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
		Return(interfaces.EntryRef{ID: testResourceID, Kind: "transaction"}, nil).Once()

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid2", 0, "create_income", "transaction", mock.AnythingOfType("usecases.WriteFn")).
		RunAndReturn(func(ctx context.Context, userID uuid.UUID, wamid string, itemSeq int, operation, resourceKind string, write usecases.WriteFn) (usecases.IdempotentWriteResult, error) {
			id, _, err := write(ctx)
			if err != nil {
				return usecases.IdempotentWriteResult{}, err
			}
			return usecases.IdempotentWriteResult{ResourceID: id, Outcome: agent.ToolOutcomeRouted}, nil
		}).Once()

	handle := BuildRegisterIncomeTool(ledger, writer)
	assert.Equal(t, "register_income", handle.ID())

	argsJSON, _ := json.Marshal(RegisterIncomeInput{
		Wamid:         "wamid2",
		UserID:        testUserID.String(),
		AmountCents:   100000,
		Description:   "Salário",
		PaymentMethod: "transfer",
	})
	out, err := handle.Invoke(context.Background(), argsJSON)
	require.NoError(t, err)

	var result RegisterIncomeOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, testResourceID.String(), result.ResourceID)
	assert.Equal(t, "transaction", result.Kind)
	assert.False(t, result.IsReplay)
}

func TestBuildRegisterCardPurchaseToolSuccess(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	cardMgr := imocks.NewCardManager(t)
	writer := mocks.NewIdempotentWriter(t)

	cardID := uuid.MustParse("00000000-0000-0000-0000-000000000010")

	cardMgr.EXPECT().ListCards(mock.Anything, testUserID).
		Return([]interfaces.Card{{ID: cardID.String(), Nickname: "Nubank"}}, nil).Once()

	ledger.EXPECT().CreateCardPurchase(mock.Anything, mock.AnythingOfType("interfaces.RawCardPurchase")).
		Return(interfaces.EntryRef{ID: testResourceID, Kind: "card_purchase"}, nil).Once()

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid3", 0, "create_card_purchase", "card_purchase", mock.AnythingOfType("usecases.WriteFn")).
		RunAndReturn(func(ctx context.Context, userID uuid.UUID, wamid string, itemSeq int, operation, resourceKind string, write usecases.WriteFn) (usecases.IdempotentWriteResult, error) {
			id, _, err := write(ctx)
			if err != nil {
				return usecases.IdempotentWriteResult{}, err
			}
			return usecases.IdempotentWriteResult{ResourceID: id, Outcome: agent.ToolOutcomeRouted}, nil
		}).Once()

	handle := BuildRegisterCardPurchaseTool(ledger, cardMgr, writer)
	assert.Equal(t, "register_card_purchase", handle.ID())

	argsJSON, _ := json.Marshal(RegisterCardPurchaseInput{
		Wamid:             "wamid3",
		UserID:            testUserID.String(),
		CardNickname:      "Nubank",
		TotalAmountCents:  30000,
		InstallmentsTotal: 3,
		Description:       "Notebook",
	})
	out, err := handle.Invoke(context.Background(), argsJSON)
	require.NoError(t, err)

	var result RegisterCardPurchaseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, testResourceID.String(), result.ResourceID)
	assert.Equal(t, "card_purchase", result.Kind)
	assert.False(t, result.IsReplay)
}

func TestBuildRegisterCardPurchaseToolInstallmentsTooHigh(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	cardMgr := imocks.NewCardManager(t)
	writer := mocks.NewIdempotentWriter(t)

	handle := BuildRegisterCardPurchaseTool(ledger, cardMgr, writer)
	argsJSON, _ := json.Marshal(RegisterCardPurchaseInput{
		Wamid:             "wamid-h3",
		UserID:            testUserID.String(),
		CardNickname:      "Nubank",
		TotalAmountCents:  100000,
		InstallmentsTotal: 25,
		Description:       "Compra inválida",
	})
	_, err := handle.Invoke(context.Background(), argsJSON)
	require.Error(t, err)
}

func TestBuildRegisterCardPurchaseToolCardNotFound(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	cardMgr := imocks.NewCardManager(t)
	writer := mocks.NewIdempotentWriter(t)

	cardMgr.EXPECT().ListCards(mock.Anything, testUserID).
		Return([]interfaces.Card{}, nil).Once()

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid3", 0, "create_card_purchase", "card_purchase", mock.AnythingOfType("usecases.WriteFn")).
		RunAndReturn(func(ctx context.Context, userID uuid.UUID, wamid string, itemSeq int, operation, resourceKind string, write usecases.WriteFn) (usecases.IdempotentWriteResult, error) {
			_, _, err := write(ctx)
			return usecases.IdempotentWriteResult{}, err
		}).Once()

	handle := BuildRegisterCardPurchaseTool(ledger, cardMgr, writer)
	argsJSON, _ := json.Marshal(RegisterCardPurchaseInput{
		Wamid:             "wamid3",
		UserID:            testUserID.String(),
		CardNickname:      "CartaoInexistente",
		TotalAmountCents:  30000,
		InstallmentsTotal: 1,
		Description:       "Compra",
	})
	_, err := handle.Invoke(context.Background(), argsJSON)
	require.Error(t, err)
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
			{Kind: "transaction", ID: testResourceID.String(), RefMonth: "2026-06", AmountCents: 5000, Direction: "outcome", Description: "Almoço", CreatedAt: time.Now()},
		}, nil).Once()

	handle := BuildQueryMonthTool(ledger)
	assert.Equal(t, "query_month", handle.ID())

	argsJSON, _ := json.Marshal(QueryMonthInput{
		UserID:   testUserID.String(),
		RefMonth: "2026-06",
	})
	out, err := handle.Invoke(context.Background(), argsJSON)
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
	argsJSON, _ := json.Marshal(QueryMonthInput{UserID: testUserID.String(), RefMonth: "2026-06"})
	_, err := handle.Invoke(context.Background(), argsJSON)
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

	argsJSON, _ := json.Marshal(QueryPlanInput{UserID: testUserID.String(), Competence: "2026-06"})
	out, err := handle.Invoke(context.Background(), argsJSON)
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
		UserID:     testUserID.String(),
		Competence: "2026-06",
		RootSlug:   "moradia",
		Percentage: 30,
	})
	out, err := handle.Invoke(context.Background(), argsJSON)
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
		UserID:     testUserID.String(),
		Competence: "2026-06",
		RootSlug:   "moradia",
		Percentage: 30,
	})
	_, err := handle.Invoke(context.Background(), argsJSON)
	require.Error(t, err)
}

func TestBuildClassifyCategoryToolSuccess(t *testing.T) {
	reader := imocks.NewCategoriesReader(t)

	reader.EXPECT().SearchDictionary(mock.Anything, "restaurante", "outcome").
		Return([]interfaces.CategoryCandidate{
			{CategoryID: testCategoryID, Path: "alimentacao/restaurante", Score: 0.95, IsAmbiguous: false},
		}, nil).Once()

	handle := BuildClassifyCategoryTool(reader)
	assert.Equal(t, "classify_category", handle.ID())

	argsJSON, _ := json.Marshal(ClassifyCategoryInput{Term: "restaurante", Kind: "outcome"})
	out, err := handle.Invoke(context.Background(), argsJSON)
	require.NoError(t, err)

	var result ClassifyCategoryOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, testCategoryID.String(), result.TopCategoryID)
	assert.Equal(t, "alimentacao/restaurante", result.TopPath)
	assert.False(t, result.IsAmbiguous)
	assert.Len(t, result.Candidates, 1)
}

func TestBuildClassifyCategoryToolAmbiguous(t *testing.T) {
	reader := imocks.NewCategoriesReader(t)

	reader.EXPECT().SearchDictionary(mock.Anything, "mercado", "outcome").
		Return([]interfaces.CategoryCandidate{
			{CategoryID: testCategoryID, Path: "alimentacao/mercado", Score: 0.8, IsAmbiguous: false},
			{CategoryID: uuid.New(), Path: "lazer/mercado", Score: 0.75, IsAmbiguous: false},
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

func TestBuildEditEntryTool(t *testing.T) {
	engine := &fakeConfirmEngine{
		startResult: wf.RunResult[workflows.ConfirmState]{Status: wf.RunStatusSuspended},
	}
	handle := BuildEditEntryTool(engine, fakeConfirmDef())
	assert.Equal(t, "edit_entry", handle.ID())

	argsJSON, _ := json.Marshal(EditEntryInput{EntryID: testResourceID.String(), EntryKind: "transaction"})
	out, err := handle.Invoke(inboundCtx(), argsJSON)
	require.NoError(t, err)

	var result EditEntryOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.NeedsConfirmation)
	assert.Equal(t, testResourceID.String(), result.TargetRef)
	assert.Equal(t, "transaction", result.TargetKind)
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
	ledger := imocks.NewTransactionsLedger(t)
	writer := mocks.NewIdempotentWriter(t)

	ledger.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
		Return(interfaces.EntryRef{ID: testResourceID, Kind: "transaction"}, nil).Once()

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid1", 0, "create_expense", "transaction", mock.AnythingOfType("usecases.WriteFn")).
		RunAndReturn(func(ctx context.Context, userID uuid.UUID, wamid string, itemSeq int, operation, resourceKind string, write usecases.WriteFn) (usecases.IdempotentWriteResult, error) {
			id, _, err := write(ctx)
			if err != nil {
				return usecases.IdempotentWriteResult{}, err
			}
			return usecases.IdempotentWriteResult{ResourceID: id, Outcome: agent.ToolOutcomeRouted}, nil
		}).Once()

	handle := BuildRegisterExpenseTool(ledger, writer)
	argsJSON, _ := json.Marshal(RegisterExpenseInput{
		Wamid: "wamid1", UserID: testUserID.String(), AmountCents: 1000, Description: "café", PaymentMethod: "debit",
	})
	out, err := handle.Invoke(context.Background(), argsJSON)
	require.NoError(t, err)

	var result RegisterExpenseOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "routed", result.Outcome)
	assert.False(t, result.IsReplay)
}

func TestRegisterIncomeOutput_OutcomeField_Replay(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	writer := mocks.NewIdempotentWriter(t)

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid2", 0, "create_income", "transaction", mock.AnythingOfType("usecases.WriteFn")).
		Return(usecases.IdempotentWriteResult{ResourceID: testResourceID, Outcome: agent.ToolOutcomeReplay}, nil).Once()

	handle := BuildRegisterIncomeTool(ledger, writer)
	argsJSON, _ := json.Marshal(RegisterIncomeInput{
		Wamid: "wamid2", UserID: testUserID.String(), AmountCents: 2000, Description: "salário", PaymentMethod: "pix",
	})
	out, err := handle.Invoke(context.Background(), argsJSON)
	require.NoError(t, err)

	var result RegisterIncomeOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "replay", result.Outcome)
	assert.True(t, result.IsReplay)
}
