//go:build integration

package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	agenttools "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/tools"
	agentusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func chainGatedRegistrar(cat agentsifaces.CategoriesReader, ledger agentsifaces.TransactionsLedger, obs *fake.Provider) (*agentusecases.RegisterAttempt, *harnessStore) {
	store := newHarnessStore()
	engine := workflow.NewEngine[workflows.PendingEntryState](store, obs)
	def := workflows.BuildPendingEntryWorkflow(ledger, nil, cat, nil)
	return agentusecases.NewRegisterAttempt(cat, ledger, engine, def, obs), store
}

func chainLoadPending(t *testing.T, store *harnessStore, userID uuid.UUID) workflows.PendingEntryState {
	t.Helper()
	key := workflows.PendingEntryKey(userID.String(), "")
	snap, found, err := store.Load(context.Background(), workflows.PendingEntryWorkflowID, key)
	require.NoError(t, err)
	require.True(t, found, "register deve abrir pendência durável (gate de confirmação), sem escrita síncrona")
	st, decErr := workflow.NewCodec[workflows.PendingEntryState]().Decode(snap.State)
	require.NoError(t, decErr)
	return st
}

func TestRealLLM_CardPurchaseChain_ResolveClassifyRegister(t *testing.T) {
	provider := buildRealLLMProvider(t)

	scenarios := []struct {
		name             string
		message          string
		wamid            string
		wantInstallments int
	}{
		{
			name:             "parcelada em 3x roteia via register_expense credit_card",
			message:          "Comprei um notebook de 3000 reais em 3x no cartão Nubank. Registre essa compra.",
			wamid:            "wamid-chain-parcelada",
			wantInstallments: 3,
		},
		{
			name:             "à vista roteia via register_expense credit_card com installments=1",
			message:          "Comprei um teclado de 300 reais à vista no cartão Nubank. Registre essa compra.",
			wamid:            "wamid-chain-avista",
			wantInstallments: 1,
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			obs := fake.NewProvider()

			userID := uuid.New()
			cardID := uuid.New()
			rootID := uuid.New()
			leafID := uuid.New()

			var resolveCalled, searchCalled bool

			cardMock := imocks.NewCardManager(t)
			cardMock.EXPECT().ResolveCardByNickname(mock.Anything, mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.Card, error) {
					resolveCalled = true
					return agentsifaces.Card{ID: cardID.String(), UserID: userID.String(), Nickname: "Nubank", Bank: "Nubank", DueDay: 1}, nil
				}).Maybe()

			catMock := imocks.NewCategoriesReader(t)
			catMock.EXPECT().SearchDictionary(mock.Anything, mock.Anything, "expense").
				RunAndReturn(func(_ context.Context, _, _ string) (agentsifaces.CategorySearchResult, error) {
					searchCalled = true
					return agentsifaces.CategorySearchResult{
						Outcome: agentsifaces.ClassifyOutcomeMatched,
						Version: 1,
						Candidates: []agentsifaces.CategoryCandidate{{
							CategoryID:     leafID,
							RootCategoryID: rootID,
							Path:           "Custo Fixo > Eletrônicos",
							Score:          0.95,
							Confidence:     "high",
							MatchQuality:   "exact",
							SignalType:     "canonical_name",
							MatchedTerm:    "eletrônicos",
						}},
					}, nil
				}).Maybe()
			catMock.EXPECT().ResolveForWrite(mock.Anything, mock.Anything).
				Return(agentsifaces.CategoryWriteDecision{
					RootCategoryID:   rootID,
					SubcategoryID:    leafID,
					Path:             "Custo Fixo > Eletrônicos",
					RootSlug:         "custo-fixo",
					SubcategorySlug:  "eletronicos",
					EditorialVersion: 1,
				}, nil).Maybe()

			ledgerMock := imocks.NewTransactionsLedger(t)

			registrar, store := chainGatedRegistrar(catMock, ledgerMock, obs)

			tools := []tool.ToolHandle{
				agenttools.BuildResolveCardTool(cardMock),
				agenttools.BuildRegisterExpenseTool(registrar),
			}

			a := BuildMeControlaAgent(provider, tools, nil, obs)

			ctx := agent.WithToolInvocationContext(context.Background(), userID.String(), sc.wamid, 0)
			ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
			defer cancel()

			result, err := a.Execute(ctx, agent.Request{
				AgentID: MecontrolaAgentID,
				Messages: []llm.Message{
					{Role: "user", Content: sc.message},
				},
				MaxTokens: 1024,
			})
			require.NoError(t, err)
			t.Logf("resposta do agente: %s", result.Content)

			require.True(t, resolveCalled, "agente deve consultar os cartões do usuário via resolve_card")
			require.True(t, searchCalled, "a categoria deve ser classificada deterministicamente pela ferramenta, não pelo LLM")

			st := chainLoadPending(t, store, userID)
			require.Equal(t, workflows.PendingOpRegisterExpense, st.OperationKind)
			require.Equal(t, "credit_card", st.PaymentMethod, "compra no crédito deve rotear via register_expense com paymentMethod=credit_card")
			require.NotNil(t, st.CardID, "register_expense deve receber o cardId resolvido pelo resolve_card")
			require.Equal(t, cardID, *st.CardID, "cardId deve ser o resolvido pelo resolve_card")
			require.Equal(t, sc.wantInstallments, st.Installments, "installments deve refletir o parcelamento informado")
			require.Len(t, st.Candidates, 1)
			require.Equal(t, rootID, st.Candidates[0].RootCategoryID, "raiz classificada deterministicamente")
			require.Equal(t, leafID, st.Candidates[0].SubcategoryID, "folha classificada deterministicamente")
		})
	}
}

func TestRealLLM_RegisterOpensConfirmationGate(t *testing.T) {
	provider := buildRealLLMProvider(t)

	obs := fake.NewProvider()
	userID := uuid.New()
	rootID := uuid.New()
	leafID := uuid.New()

	catMock := imocks.NewCategoriesReader(t)
	catMock.EXPECT().SearchDictionary(mock.Anything, mock.Anything, "expense").
		Return(agentsifaces.CategorySearchResult{
			Outcome: agentsifaces.ClassifyOutcomeMatched,
			Version: 9,
			Candidates: []agentsifaces.CategoryCandidate{{
				CategoryID:     leafID,
				RootCategoryID: rootID,
				Path:           "Custo Fixo > Supermercado",
				Score:          0.95,
				Confidence:     "high",
				MatchQuality:   "exact",
				SignalType:     "canonical_name",
				MatchedTerm:    "supermercado",
			}},
		}, nil).Maybe()
	catMock.EXPECT().ResolveForWrite(mock.Anything, mock.Anything).
		Return(agentsifaces.CategoryWriteDecision{
			RootCategoryID:   rootID,
			SubcategoryID:    leafID,
			Path:             "Custo Fixo > Supermercado",
			RootSlug:         "custo-fixo",
			SubcategorySlug:  "supermercado",
			EditorialVersion: 9,
		}, nil).Maybe()

	ledgerMock := imocks.NewTransactionsLedger(t)

	registrar, store := chainGatedRegistrar(catMock, ledgerMock, obs)

	tools := []tool.ToolHandle{
		agenttools.BuildRegisterExpenseTool(registrar),
		agenttools.BuildClassifyCategoryTool(catMock),
	}
	a := BuildMeControlaAgent(provider, tools, nil, obs)

	ctx := agent.WithToolInvocationContext(context.Background(), userID.String(), "wamid-gate-chain", 0)
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 150,00 no mercado hoje, no pix. Registre esse lançamento."},
		},
		MaxTokens: 1024,
	})
	require.NoError(t, err)
	t.Logf("resposta do agente: %s", result.Content)

	lower := strings.ToLower(result.Content)
	require.Contains(t, lower, "confirma", "o agente deve relayar a pergunta de confirmação ao usuário, não improvisar erro")
	require.NotContains(t, lower, "dificuldade", "não deve reportar falha quando o gate apenas abriu a confirmação")

	st := chainLoadPending(t, store, userID)
	require.Equal(t, workflows.PendingOpRegisterExpense, st.OperationKind)
	require.Equal(t, int64(15000), st.AmountCents, "valor original deve ser preservado no gate")
	require.Equal(t, "pix", st.PaymentMethod, "forma de pagamento original deve ser preservada")
	require.Equal(t, workflows.PendingStatusActive, st.Status, "O-07/RF-38: escrita não persiste antes da confirmação")
}

func TestRealLLM_QueryCardInvoiceChain_C4(t *testing.T) {
	provider := buildRealLLMProvider(t)

	const iterations = 10
	const minPass = 8

	passes := 0
	for i := 1; i <= iterations; i++ {
		obs := fake.NewProvider()
		userID := uuid.New()
		cardID := uuid.New()

		var capturedCardID uuid.UUID

		cardMock := imocks.NewCardManager(t)
		cardMock.EXPECT().ResolveCardByNickname(mock.Anything, mock.Anything, mock.AnythingOfType("string")).
			RunAndReturn(func(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.Card, error) {
				return agentsifaces.Card{ID: cardID.String(), UserID: userID.String(), Nickname: "Nubank", Bank: "Nubank", DueDay: 10}, nil
			}).Maybe()

		ledgerMock := imocks.NewTransactionsLedger(t)
		ledgerMock.EXPECT().GetCardInvoice(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
			RunAndReturn(func(_ context.Context, cid uuid.UUID, _ string) (agentsifaces.CardInvoice, error) {
				capturedCardID = cid
				return agentsifaces.CardInvoice{
					ID:              uuid.New(),
					UserID:          userID,
					CardID:          cardID,
					RefMonth:        "2026-07",
					ClosingAt:       time.Now().Add(10 * 24 * time.Hour),
					DueAt:           time.Now().Add(17 * 24 * time.Hour),
					ItemsTotalCents: 45000,
					Items:           []agentsifaces.CardInvoiceItem{},
				}, nil
			}).Maybe()

		tools := []tool.ToolHandle{
			agenttools.BuildResolveCardTool(cardMock),
			agenttools.BuildQueryCardInvoiceTool(ledgerMock),
		}

		a := BuildMeControlaAgent(provider, tools, nil, obs)
		ctx := agent.WithToolInvocationContext(context.Background(), userID.String(), "wamid-c4-chain", 0)
		ctx, cancel := context.WithTimeout(ctx, 120*time.Second)

		result, err := a.Execute(ctx, agent.Request{
			AgentID: MecontrolaAgentID,
			Messages: []llm.Message{
				{Role: "user", Content: "quanto está minha fatura do cartão nubank?"},
			},
			MaxTokens: 1024,
		})
		cancel()
		require.NoError(t, err)

		lower := strings.ToLower(result.Content)
		require.NotContainsf(t, lower, "nubank-", "anti-alucinação: resposta não deve conter cardId bruto fabricado")

		if capturedCardID == cardID {
			passes++
		} else {
			t.Logf("C4 iteração %d não roteou resolve_card→query_card_invoice: %s", i, result.Content)
		}
	}

	t.Logf("C4 gate de confiabilidade: %d/%d roteou resolve_card→query_card_invoice", passes, iterations)
	require.GreaterOrEqualf(t, passes, minPass,
		"RF-05/RF-32a: C4 deve rotear resolve_card→query_card_invoice em pelo menos %d de %d execuções (cardId sempre de resolve_card)", minPass, iterations)
}

func TestRealLLM_LastTransactionChain_C5(t *testing.T) {
	provider := buildRealLLMProvider(t)

	obs := fake.NewProvider()
	userID := uuid.New()
	knownTxID := "txn-known-" + uuid.New().String()

	var capturedTxID string

	ledgerMock := imocks.NewTransactionsLedger(t)
	ledgerMock.EXPECT().GetMonthlySummary(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string")).
		Return(agentsifaces.MonthlySummary{
			RefMonth:     "2026-07",
			IncomeCents:  500000,
			OutcomeCents: 32000,
			TotalCents:   468000,
		}, nil).Maybe()
	ledgerMock.EXPECT().ListMonthlyEntries(mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("string"), mock.Anything, mock.Anything).
		Return([]agentsifaces.MonthlyEntry{
			{
				Kind:        agentsifaces.EntryKindTransaction,
				ID:          knownTxID,
				RefMonth:    "2026-07",
				AmountCents: 32000,
				Direction:   "outcome",
				Description: "Supermercado Extra",
				CreatedAt:   time.Now().Add(-2 * time.Hour),
			},
		}, nil).Maybe()
	ledgerMock.EXPECT().GetTransaction(mock.Anything, mock.AnythingOfType("string")).
		RunAndReturn(func(_ context.Context, txID string) (agentsifaces.Entry, error) {
			capturedTxID = txID
			return agentsifaces.Entry{
				Kind:                    agentsifaces.EntryKindTransaction,
				ID:                      txID,
				UserID:                  userID.String(),
				Direction:               "outcome",
				PaymentMethod:           "debit",
				AmountCents:             32000,
				Description:             "Supermercado Extra",
				CategoryNameSnapshot:    "Custo Fixo",
				SubcategoryNameSnapshot: "Supermercado",
				RefMonth:                "2026-07",
				OccurredAt:              time.Now().Add(-2 * time.Hour),
			}, nil
		}).Maybe()

	tools := []tool.ToolHandle{
		agenttools.BuildQueryMonthTool(ledgerMock),
		agenttools.BuildGetTransactionTool(ledgerMock),
	}

	a := BuildMeControlaAgent(provider, tools, nil, obs)
	ctx := agent.WithToolInvocationContext(context.Background(), userID.String(), "wamid-c5-chain", 0)
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "qual foi a minha última transação?"},
		},
		MaxTokens: 1024,
	})
	require.NoError(t, err)
	t.Logf("C5 resposta: %s", result.Content)

	require.Equal(t, knownTxID, capturedTxID, "C5: get_transaction deve ser chamado com o ID retornado por query_month")

	lower := strings.ToLower(result.Content)
	require.True(t,
		strings.Contains(lower, "custo fixo") && strings.Contains(lower, "supermercado"),
		"RF-06/D-11: resposta deve exibir categoryNameSnapshot E subcategoryNameSnapshot via get_transaction (o formato literal '>' é best-effort de apresentação, ver D-11); resposta=%s", result.Content,
	)
}
