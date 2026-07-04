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
	obs := fake.NewProvider()

	userID := uuid.New()
	cardID := uuid.New()
	rootID := uuid.New()
	leafID := uuid.New()

	var resolveCalled, searchCalled bool
	var captured agentsifaces.RawCardPurchase

	cardMock := imocks.NewCardManager(t)
	cardMock.EXPECT().ResolveCardByNickname(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.Card, error) {
			resolveCalled = true
			return agentsifaces.Card{ID: cardID.String(), UserID: userID.String(), Nickname: "Nubank", Bank: "Nubank", DueDay: 1}, nil
		}).Maybe()

	catMock := imocks.NewCategoriesReader(t)
	catMock.EXPECT().SearchDictionary(mock.Anything, mock.Anything, "expense").
		RunAndReturn(func(_ context.Context, _, _ string) ([]agentsifaces.CategoryCandidate, error) {
			searchCalled = true
			return []agentsifaces.CategoryCandidate{{CategoryID: leafID, RootCategoryID: rootID, Path: "Custo Fixo > Eletrônicos", Score: 0.95}}, nil
		}).Maybe()

	ledgerMock := imocks.NewTransactionsLedger(t)
	ledgerMock.EXPECT().CreateCardPurchase(mock.Anything, mock.AnythingOfType("interfaces.RawCardPurchase")).
		RunAndReturn(func(_ context.Context, in agentsifaces.RawCardPurchase) (agentsifaces.EntryRef, error) {
			captured = in
			return agentsifaces.EntryRef{ID: uuid.New(), Kind: "card_purchase"}, nil
		}).Maybe()

	registrar := agentusecases.NewRegisterEntry(catMock, ledgerMock, chainFakeWriter{}, obs)

	tools := []tool.ToolHandle{
		agenttools.BuildResolveCardTool(cardMock),
		agenttools.BuildRegisterCardPurchaseTool(registrar),
	}

	a := BuildMeControlaAgent(provider, tools, nil, obs)

	ctx := agent.WithToolInvocationContext(context.Background(), userID.String(), "wamid-chain-1", 0)
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Comprei um notebook de 3000 reais em 3x no cartão Nubank. Registre essa compra."},
		},
		MaxTokens: 1024,
	})
	require.NoError(t, err)
	t.Logf("resposta do agente: %s", result.Content)

	require.True(t, resolveCalled, "agente deve consultar os cartões do usuário via resolve_card")
	require.True(t, searchCalled, "a categoria deve ser classificada deterministicamente pela ferramenta, não pelo LLM")
	require.Equal(t, cardID, captured.CardID, "register_card_purchase deve receber o cardId resolvido pelo resolve_card")
	require.Equal(t, rootID, captured.CategoryID, "category_id gravado deve ser a raiz classificada deterministicamente")
	require.NotNil(t, captured.SubcategoryID, "subcategory_id (detalhe) é obrigatório na persistência")
	require.Equal(t, leafID, *captured.SubcategoryID, "subcategory_id gravado deve ser a folha classificada deterministicamente")
}
