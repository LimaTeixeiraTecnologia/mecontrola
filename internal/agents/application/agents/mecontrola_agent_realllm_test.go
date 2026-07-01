//go:build integration

package agents

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/scorers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

func buildRealLLMProvider(t *testing.T) llm.Provider {
	t.Helper()
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" || os.Getenv("RUN_REAL_LLM") != "1" {
		t.Skip("RUN_REAL_LLM=1 e OPENROUTER_API_KEY obrigatórios")
	}
	baseURL := "https://openrouter.ai"
	client, err := httpclient.NewClient(fake.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter"),
		httpclient.WithTimeout(30*time.Second),
	)
	require.NoError(t, err)
	return llm.NewOpenRouterProvider(client, llm.Config{
		Model:          "openai/gpt-4o-mini",
		BaseURL:        baseURL,
		APIKey:         apiKey,
		HTTPReferer:    "https://github.com/LimaTeixeiraTecnologia/mecontrola",
		XTitle:         "mecontrola-integration-test",
		MaxTokens:      512,
		Temperature:    0,
		RequestTimeout: 30 * time.Second,
	}, fake.NewProvider())
}

func buildFakeRegisterExpenseTool() tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"wamid":         map[string]any{"type": "string"},
				"itemSeq":       map[string]any{"type": "integer"},
				"userId":        map[string]any{"type": "string"},
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"occurredAt":    map[string]any{"type": "string"},
				"categoryId":    map[string]any{"type": "string"},
				"subcategoryId": map[string]any{"type": "string"},
			},
			"required":             []string{"wamid", "itemSeq", "userId", "amountCents", "description", "paymentMethod"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"resourceId": map[string]any{"type": "string"},
				"kind":       map[string]any{"type": "string"},
				"isReplay":   map[string]any{"type": "boolean"},
			},
			"required":             []string{"resourceId", "kind", "isReplay"},
			"additionalProperties": false,
		},
	}
	type input struct {
		Wamid         string `json:"wamid"`
		ItemSeq       int    `json:"itemSeq"`
		UserID        string `json:"userId"`
		AmountCents   int64  `json:"amountCents"`
		Description   string `json:"description"`
		PaymentMethod string `json:"paymentMethod"`
	}
	type output struct {
		ResourceID string `json:"resourceId"`
		Kind       string `json:"kind"`
		IsReplay   bool   `json:"isReplay"`
	}
	return tool.NewTool[input, output]("register_expense", "Registra um lançamento de despesa no ledger financeiro do usuário.", in, out,
		func(_ context.Context, in input) (output, error) {
			return output{
				ResourceID: uuid.New().String(),
				Kind:       "transaction",
				IsReplay:   false,
			}, nil
		},
	)
}

func buildFakeQueryMonthTool() tool.ToolHandle {
	in := llm.Schema{
		Name:   "query_month_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"userId":   map[string]any{"type": "string"},
				"refMonth": map[string]any{"type": "string"},
				"cursor":   map[string]any{"type": "string"},
				"limit":    map[string]any{"type": "integer"},
			},
			"required":             []string{"userId"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "query_month_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"refMonth":     map[string]any{"type": "string"},
				"incomeCents":  map[string]any{"type": "integer"},
				"outcomeCents": map[string]any{"type": "integer"},
				"totalCents":   map[string]any{"type": "integer"},
				"entries":      map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			},
			"required":             []string{"refMonth", "incomeCents", "outcomeCents", "totalCents", "entries"},
			"additionalProperties": false,
		},
	}
	type input struct {
		UserID   string `json:"userId"`
		RefMonth string `json:"refMonth,omitempty"`
	}
	type output struct {
		RefMonth     string `json:"refMonth"`
		IncomeCents  int64  `json:"incomeCents"`
		OutcomeCents int64  `json:"outcomeCents"`
		TotalCents   int64  `json:"totalCents"`
		Entries      []any  `json:"entries"`
	}
	return tool.NewTool[input, output]("query_month", "Consulta o resumo e os lançamentos do mês financeiro do usuário.", in, out,
		func(_ context.Context, in input) (output, error) {
			refMonth := in.RefMonth
			if refMonth == "" {
				refMonth = time.Now().Format("2006-01")
			}
			return output{
				RefMonth:     refMonth,
				IncomeCents:  500000,
				OutcomeCents: 320000,
				TotalCents:   180000,
				Entries:      []any{},
			}, nil
		},
	)
}

func TestRealLLM_ToolCalling_RegisterExpense(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()
	userID := uuid.New().String()

	tools := []tool.ToolHandle{
		buildFakeRegisterExpenseTool(),
		buildFakeQueryMonthTool(),
	}

	a := BuildMeControlaAgent(provider, tools, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "meu userId é " + userID + " e o wamid é wamid-test-001. gastei 50 reais no almoço hoje. paymentMethod: debit"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("resposta do agente: %s", result.Content)
}

func TestRealLLM_ToolCalling_QueryMonth(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()
	userID := uuid.New().String()

	tools := []tool.ToolHandle{
		buildFakeRegisterExpenseTool(),
		buildFakeQueryMonthTool(),
	}

	a := BuildMeControlaAgent(provider, tools, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "meu userId é " + userID + ". quanto gastei esse mês?"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("resposta do agente: %s", result.Content)
}

func TestRealLLM_Scorer_CategorizationLLMJudged(t *testing.T) {
	provider := buildRealLLMProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sc := scorers.NewCategorizationScorer(provider)

	require.Equal(t, scorer.ScorerKindLLMJudged, sc.Kind())

	sample := scorer.RunSample{
		Input:  "gastei no mercado",
		Output: "✅ Registrei R$ 100,00 na categoria Alimentação. Lançamento confirmado para o mês atual.",
	}

	result, err := sc.Score(ctx, sample)

	require.NoError(t, err)
	require.GreaterOrEqual(t, result.Score, 0.0)
	require.LessOrEqual(t, result.Score, 1.0)
	require.NotEmpty(t, result.Reason)
	t.Logf("score=%.2f reason=%s", result.Score, result.Reason)
}
