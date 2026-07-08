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
)

var (
	testCardID    = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	testRecID     = uuid.MustParse("00000000-0000-0000-0000-000000000012")
	testCatID     = uuid.MustParse("00000000-0000-0000-0000-000000000013")
	testInvoiceID = uuid.MustParse("00000000-0000-0000-0000-000000000014")
	testItemID    = uuid.MustParse("00000000-0000-0000-0000-000000000015")
	errBinding    = errors.New("binding error")
)

func noIdentityCtx() context.Context {
	return context.Background()
}

func TestListCardsTool_Success(t *testing.T) {
	cards := imocks.NewCardManager(t)
	cards.EXPECT().ListCards(mock.Anything, testUserID).Return([]interfaces.Card{
		{ID: testCardID.String(), Nickname: "Nubank", Bank: "nubank", DueDay: 10, ClosingDay: 3, BestPurchaseDay: 4},
	}, nil).Once()

	h := BuildListCardsTool(cards)
	assert.Equal(t, "list_cards", h.ID())

	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(ListCardsInput{}))
	require.NoError(t, err)

	var res ListCardsOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Len(t, res.Cards, 1)
	assert.Equal(t, testCardID.String(), res.Cards[0].ID)
	assert.Equal(t, "Nubank", res.Cards[0].Nickname)
}

func TestListCardsTool_BindingError(t *testing.T) {
	cards := imocks.NewCardManager(t)
	cards.EXPECT().ListCards(mock.Anything, testUserID).Return(nil, errBinding).Once()

	h := BuildListCardsTool(cards)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(ListCardsInput{}))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBinding)
}

func TestListCardsTool_NoIdentity(t *testing.T) {
	cards := imocks.NewCardManager(t)
	h := BuildListCardsTool(cards)
	_, err := h.Invoke(noIdentityCtx(), mustMarshal(ListCardsInput{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identidade não disponível")
}

func TestGetCardTool_Success(t *testing.T) {
	cards := imocks.NewCardManager(t)
	cards.EXPECT().GetCard(mock.Anything, testCardID, testUserID).Return(interfaces.Card{
		ID: testCardID.String(), Nickname: "Inter", Bank: "inter", DueDay: 5, ClosingDay: 28, BestPurchaseDay: 1,
	}, nil).Once()

	h := BuildGetCardTool(cards)
	assert.Equal(t, "get_card", h.ID())

	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(GetCardInput{CardID: testCardID.String()}))
	require.NoError(t, err)

	var res GetCardOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Equal(t, testCardID.String(), res.ID)
	assert.Equal(t, "Inter", res.Nickname)
}

func TestGetCardTool_BindingError(t *testing.T) {
	cards := imocks.NewCardManager(t)
	cards.EXPECT().GetCard(mock.Anything, testCardID, testUserID).Return(interfaces.Card{}, errBinding).Once()

	h := BuildGetCardTool(cards)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(GetCardInput{CardID: testCardID.String()}))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBinding)
}

func TestGetCardTool_InvalidCardID(t *testing.T) {
	cards := imocks.NewCardManager(t)
	h := BuildGetCardTool(cards)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(GetCardInput{CardID: "not-a-uuid"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cardId inválido")
}

func TestGetCardTool_NoIdentity(t *testing.T) {
	cards := imocks.NewCardManager(t)
	h := BuildGetCardTool(cards)
	_, err := h.Invoke(noIdentityCtx(), mustMarshal(GetCardInput{CardID: testCardID.String()}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identidade não disponível")
}

func TestQueryCardInvoiceTool_Success(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	ledger.EXPECT().GetCardInvoice(mock.Anything, testCardID, "2026-01").Return(interfaces.CardInvoice{
		ID:              testInvoiceID,
		CardID:          testCardID,
		RefMonth:        "2026-01",
		ClosingAt:       time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
		DueAt:           time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
		ItemsTotalCents: 5000,
		Items: []interfaces.CardInvoiceItem{
			{ID: testItemID, InvoiceID: testInvoiceID, RefMonth: "2026-01", InstallmentIndex: 1, AmountCents: 5000},
		},
	}, nil).Once()

	h := BuildQueryCardInvoiceTool(ledger)
	assert.Equal(t, "query_card_invoice", h.ID())

	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(QueryCardInvoiceInput{CardID: testCardID.String(), RefMonth: "2026-01"}))
	require.NoError(t, err)

	var res QueryCardInvoiceOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Equal(t, testInvoiceID.String(), res.ID)
	assert.Equal(t, "2026-01", res.RefMonth)
	assert.Equal(t, int64(5000), res.ItemsTotalCents)
	assert.Len(t, res.Items, 1)
}

func TestQueryCardInvoiceTool_DefaultRefMonth(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	ledger.EXPECT().GetCardInvoice(mock.Anything, testCardID, mock.AnythingOfType("string")).Return(interfaces.CardInvoice{
		ID:       testInvoiceID,
		CardID:   testCardID,
		RefMonth: time.Now().Format("2006-01"),
	}, nil).Once()

	h := BuildQueryCardInvoiceTool(ledger)
	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(QueryCardInvoiceInput{CardID: testCardID.String()}))
	require.NoError(t, err)
	require.NotEmpty(t, out)
}

func TestQueryCardInvoiceTool_InvalidCardID(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	h := BuildQueryCardInvoiceTool(ledger)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(QueryCardInvoiceInput{CardID: "bad"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cardId inválido")
}

func TestQueryCardInvoiceTool_BindingError(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	ledger.EXPECT().GetCardInvoice(mock.Anything, testCardID, "2026-01").Return(interfaces.CardInvoice{}, errBinding).Once()

	h := BuildQueryCardInvoiceTool(ledger)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(QueryCardInvoiceInput{CardID: testCardID.String(), RefMonth: "2026-01"}))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBinding)
}

func TestBestPurchaseDayTool_Success(t *testing.T) {
	cards := imocks.NewCardManager(t)
	cards.EXPECT().BestPurchaseDay(mock.Anything, "nubank", 10).Return(interfaces.BestPurchaseDay{
		ClosingDay: 3, BestPurchaseDay: 4,
	}, nil).Once()

	h := BuildBestPurchaseDayTool(cards)
	assert.Equal(t, "best_purchase_day", h.ID())

	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(BestPurchaseDayInput{Bank: "nubank", DueDay: 10}))
	require.NoError(t, err)

	var res BestPurchaseDayOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Equal(t, 3, res.ClosingDay)
	assert.Equal(t, 4, res.BestPurchaseDay)
}

func TestBestPurchaseDayTool_BindingError(t *testing.T) {
	cards := imocks.NewCardManager(t)
	cards.EXPECT().BestPurchaseDay(mock.Anything, "nubank", 10).Return(interfaces.BestPurchaseDay{}, errBinding).Once()

	h := BuildBestPurchaseDayTool(cards)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(BestPurchaseDayInput{Bank: "nubank", DueDay: 10}))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBinding)
}

func TestSearchTransactionsTool_Success(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	ledger.EXPECT().SearchTransactions(mock.Anything, testUserID, "almoço", "2026-01", 20).Return([]interfaces.Entry{
		{Kind: interfaces.EntryKindTransaction, ID: "tx-1", RefMonth: "2026-01", AmountCents: 3000, Direction: "outcome", Description: "Almoço"},
	}, nil).Once()

	h := BuildSearchTransactionsTool(ledger)
	assert.Equal(t, "search_transactions", h.ID())

	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(SearchTransactionsInput{Query: "almoço", RefMonth: "2026-01"}))
	require.NoError(t, err)

	var res SearchTransactionsOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Len(t, res.Entries, 1)
	assert.Equal(t, "tx-1", res.Entries[0].ID)
}

func TestSearchTransactionsTool_DefaultRefMonthAndLimit(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	ledger.EXPECT().SearchTransactions(mock.Anything, testUserID, "café", mock.AnythingOfType("string"), 20).Return(nil, nil).Once()

	h := BuildSearchTransactionsTool(ledger)
	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(SearchTransactionsInput{Query: "café"}))
	require.NoError(t, err)
	var res SearchTransactionsOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Empty(t, res.Entries)
}

func TestSearchTransactionsTool_BindingError(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	ledger.EXPECT().SearchTransactions(mock.Anything, testUserID, "x", mock.Anything, mock.Anything).Return(nil, errBinding).Once()

	h := BuildSearchTransactionsTool(ledger)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(SearchTransactionsInput{Query: "x"}))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBinding)
}

func TestSearchTransactionsTool_NoIdentity(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	h := BuildSearchTransactionsTool(ledger)
	_, err := h.Invoke(noIdentityCtx(), mustMarshal(SearchTransactionsInput{Query: "x"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identidade não disponível")
}

func TestListRecurrencesTool_Success(t *testing.T) {
	recurrences := imocks.NewRecurrenceManager(t)
	endedAt := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	recurrences.EXPECT().ListRecurrences(mock.Anything, true, "", 50).Return([]interfaces.Recurrence{
		{
			ID:                   testRecID,
			Direction:            "outcome",
			PaymentMethod:        "debit",
			AmountCents:          10000,
			Description:          "Netflix",
			CategoryID:           testCatID,
			CategoryNameSnapshot: "Assinaturas",
			Frequency:            "monthly",
			DayOfMonth:           5,
			StartedAt:            time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
			EndedAt:              &endedAt,
		},
	}, nil).Once()

	h := BuildListRecurrencesTool(recurrences)
	assert.Equal(t, "list_recurrences", h.ID())

	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(ListRecurrencesInput{ActiveOnly: true}))
	require.NoError(t, err)

	var res ListRecurrencesOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Len(t, res.Recurrences, 1)
	assert.Equal(t, testRecID.String(), res.Recurrences[0].ID)
	assert.Equal(t, "Netflix", res.Recurrences[0].Description)
	assert.NotNil(t, res.Recurrences[0].EndedAt)
}

func TestListRecurrencesTool_DefaultLimit(t *testing.T) {
	recurrences := imocks.NewRecurrenceManager(t)
	recurrences.EXPECT().ListRecurrences(mock.Anything, false, "", 50).Return(nil, nil).Once()

	h := BuildListRecurrencesTool(recurrences)
	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(ListRecurrencesInput{}))
	require.NoError(t, err)
	var res ListRecurrencesOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Empty(t, res.Recurrences)
}

func TestListRecurrencesTool_BindingError(t *testing.T) {
	recurrences := imocks.NewRecurrenceManager(t)
	recurrences.EXPECT().ListRecurrences(mock.Anything, false, "", 50).Return(nil, errBinding).Once()

	h := BuildListRecurrencesTool(recurrences)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(ListRecurrencesInput{}))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBinding)
}

func TestGetTransactionTool_Success(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	ledger.EXPECT().GetTransaction(mock.Anything, "tx-abc").Return(interfaces.Entry{
		Kind:                    interfaces.EntryKindTransaction,
		ID:                      "tx-abc",
		UserID:                  testUserID.String(),
		Direction:               "outcome",
		PaymentMethod:           "debit",
		AmountCents:             2500,
		Description:             "Café",
		CategoryID:              testCatID.String(),
		CategoryNameSnapshot:    "Alimentação",
		SubcategoryNameSnapshot: "Supermercado",
		RefMonth:                "2026-01",
		OccurredAt:              time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		Version:                 1,
	}, nil).Once()

	h := BuildGetTransactionTool(ledger)
	assert.Equal(t, "get_transaction", h.ID())

	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(GetTransactionInput{TxID: "tx-abc"}))
	require.NoError(t, err)

	var res GetTransactionOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Equal(t, "tx-abc", res.ID)
	assert.Equal(t, "transaction", res.Kind)
	assert.Equal(t, int64(2500), res.AmountCents)
	assert.Equal(t, "Alimentação", res.CategoryNameSnapshot)
	assert.Equal(t, "Supermercado", res.SubcategoryNameSnapshot)
}

func TestGetTransactionTool_NoSubcategory(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	ledger.EXPECT().GetTransaction(mock.Anything, "tx-xyz").Return(interfaces.Entry{
		Kind:                    interfaces.EntryKindTransaction,
		ID:                      "tx-xyz",
		UserID:                  testUserID.String(),
		Direction:               "income",
		PaymentMethod:           "transfer",
		AmountCents:             10000,
		Description:             "Salário",
		CategoryID:              testCatID.String(),
		CategoryNameSnapshot:    "Renda",
		SubcategoryNameSnapshot: "",
		RefMonth:                "2026-01",
		OccurredAt:              time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
		Version:                 1,
	}, nil).Once()

	h := BuildGetTransactionTool(ledger)

	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(GetTransactionInput{TxID: "tx-xyz"}))
	require.NoError(t, err)

	var res GetTransactionOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Equal(t, "tx-xyz", res.ID)
	assert.Equal(t, "Renda", res.CategoryNameSnapshot)
	assert.Equal(t, "", res.SubcategoryNameSnapshot)
}

func TestGetTransactionTool_BindingError(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	ledger.EXPECT().GetTransaction(mock.Anything, "tx-abc").Return(interfaces.Entry{}, errBinding).Once()

	h := BuildGetTransactionTool(ledger)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(GetTransactionInput{TxID: "tx-abc"}))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBinding)
}

func TestCountCardsTool_Success(t *testing.T) {
	cards := imocks.NewCardManager(t)
	cards.EXPECT().CountCards(mock.Anything, testUserID).Return(int64(3), nil).Once()

	h := BuildCountCardsTool(cards)
	assert.Equal(t, "count_cards", h.ID())

	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(CountCardsInput{}))
	require.NoError(t, err)

	var res CountCardsOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Equal(t, int64(3), res.Count)
}

func TestCountCardsTool_BindingError(t *testing.T) {
	cards := imocks.NewCardManager(t)
	cards.EXPECT().CountCards(mock.Anything, testUserID).Return(int64(0), errBinding).Once()

	h := BuildCountCardsTool(cards)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(CountCardsInput{}))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBinding)
}

func TestCountCardsTool_NoIdentity(t *testing.T) {
	cards := imocks.NewCardManager(t)
	h := BuildCountCardsTool(cards)
	_, err := h.Invoke(noIdentityCtx(), mustMarshal(CountCardsInput{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identidade não disponível")
}

func TestSuggestAllocationTool_Success(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)
	planner.EXPECT().SuggestAllocation(mock.Anything, int64(100000), mock.AnythingOfType("[]interfaces.AllocationBP")).Return([]interfaces.AllocationCents{
		{RootSlug: "moradia", BasisPoints: 3000, PlannedCents: 30000},
		{RootSlug: "alimentacao", BasisPoints: 2000, PlannedCents: 20000},
	}, nil).Once()

	h := BuildSuggestAllocationTool(planner)
	assert.Equal(t, "suggest_allocation", h.ID())

	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(SuggestAllocationInput{
		TotalCents: 100000,
		Allocations: []SuggestAllocationInputItem{
			{RootSlug: "moradia", BasisPoints: 3000},
			{RootSlug: "alimentacao", BasisPoints: 2000},
		},
	}))
	require.NoError(t, err)

	var res SuggestAllocationOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Len(t, res.Allocations, 2)
	assert.Equal(t, "moradia", res.Allocations[0].RootSlug)
	assert.Equal(t, int64(30000), res.Allocations[0].PlannedCents)
}

func TestSuggestAllocationTool_BindingError(t *testing.T) {
	planner := imocks.NewBudgetPlanner(t)
	planner.EXPECT().SuggestAllocation(mock.Anything, int64(100000), mock.AnythingOfType("[]interfaces.AllocationBP")).Return(nil, errBinding).Once()

	h := BuildSuggestAllocationTool(planner)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(SuggestAllocationInput{
		TotalCents:  100000,
		Allocations: []SuggestAllocationInputItem{{RootSlug: "moradia", BasisPoints: 3000}},
	}))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBinding)
}

func TestListCategoriesTool_Success(t *testing.T) {
	reader := imocks.NewCategoriesReader(t)
	subID := uuid.MustParse("00000000-0000-0000-0000-000000000020")
	reader.EXPECT().ListCategories(mock.Anything, testUserID).Return([]interfaces.Category{
		{
			ID:             testCatID,
			Slug:           "alimentacao",
			Name:           "Alimentação",
			Kind:           "expense",
			AllocationType: "fixed",
			Subcategories: []interfaces.Category{
				{ID: subID, Slug: "restaurante", Name: "Restaurante", Kind: "expense", AllocationType: "variable"},
			},
		},
	}, nil).Once()

	h := BuildListCategoriesTool(reader)
	assert.Equal(t, "list_categories", h.ID())

	out, err := h.Invoke(identityCtx("w1", 0), mustMarshal(ListCategoriesInput{}))
	require.NoError(t, err)

	var res ListCategoriesOutput
	require.NoError(t, json.Unmarshal(out, &res))
	assert.Len(t, res.Categories, 1)
	assert.Equal(t, testCatID.String(), res.Categories[0].ID)
	assert.Equal(t, "alimentacao", res.Categories[0].Slug)
	assert.Len(t, res.Categories[0].Subcategories, 1)
	assert.Equal(t, "restaurante", res.Categories[0].Subcategories[0].Slug)
}

func TestListCategoriesTool_BindingError(t *testing.T) {
	reader := imocks.NewCategoriesReader(t)
	reader.EXPECT().ListCategories(mock.Anything, testUserID).Return(nil, errBinding).Once()

	h := BuildListCategoriesTool(reader)
	_, err := h.Invoke(identityCtx("w1", 0), mustMarshal(ListCategoriesInput{}))
	require.Error(t, err)
	assert.ErrorIs(t, err, errBinding)
}

func TestListCategoriesTool_NoIdentity(t *testing.T) {
	reader := imocks.NewCategoriesReader(t)
	h := BuildListCategoriesTool(reader)
	_, err := h.Invoke(noIdentityCtx(), mustMarshal(ListCategoriesInput{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identidade não disponível")
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
