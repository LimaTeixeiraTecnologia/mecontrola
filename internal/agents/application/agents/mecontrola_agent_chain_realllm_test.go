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
	def := workflows.BuildPendingEntryWorkflow(ledger, nil, cat)
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
