//go:build integration

package agents

import (
	"context"
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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type chainFakeWriter struct{}

func (chainFakeWriter) Execute(ctx context.Context, _ uuid.UUID, _ string, _ int, _, _ string, write agentusecases.WriteFn) (agentusecases.IdempotentWriteResult, error) {
	id, _, err := write(ctx)
	if err != nil {
		return agentusecases.IdempotentWriteResult{}, err
	}
	return agentusecases.IdempotentWriteResult{ResourceID: id, Outcome: agent.ToolOutcomeRouted}, nil
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
			var captured agentsifaces.RawTransaction

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
						HasMore: false,
						Candidates: []agentsifaces.CategoryCandidate{{
							CategoryID:     leafID,
							RootCategoryID: rootID,
							Path:           "Custo Fixo > Eletrônicos",
							Score:          0.95,
							Confidence:     "high",
							MatchQuality:   "exact",
							SignalType:     "canonical_name",
							MatchedTerm:    "eletrônicos",
							IsAmbiguous:    false,
						}},
					}, nil
				}).Maybe()

			ledgerMock := imocks.NewTransactionsLedger(t)
			ledgerMock.EXPECT().CreateTransaction(mock.Anything, mock.AnythingOfType("interfaces.RawTransaction")).
				RunAndReturn(func(_ context.Context, in agentsifaces.RawTransaction) (agentsifaces.EntryRef, error) {
					captured = in
					return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindTransaction}, nil
				}).Maybe()

			registrar := agentusecases.NewRegisterEntry(catMock, ledgerMock, chainFakeWriter{}, obs)

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
			require.Equal(t, "credit_card", captured.PaymentMethod, "compra no crédito deve rotear via register_expense com paymentMethod=credit_card")
			require.NotNil(t, captured.CardID, "register_expense deve receber o cardId resolvido pelo resolve_card")
			require.Equal(t, cardID, *captured.CardID, "cardId gravado deve ser o resolvido pelo resolve_card")
			require.Equal(t, sc.wantInstallments, captured.Installments, "installments deve refletir o parcelamento informado")
			require.Equal(t, rootID, captured.CategoryID, "category_id gravado deve ser a raiz classificada deterministicamente")
			require.NotNil(t, captured.SubcategoryID, "subcategory_id (detalhe) é obrigatório na persistência")
			require.Equal(t, leafID, *captured.SubcategoryID, "subcategory_id gravado deve ser a folha classificada deterministicamente")
		})
	}
}
