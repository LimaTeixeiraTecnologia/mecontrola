//go:build integration

package scorers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

func buildHarnessProvider(t *testing.T) llm.Provider {
	t.Helper()
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" || os.Getenv("RUN_REAL_LLM") != "1" {
		t.Skip("RUN_REAL_LLM=1 e OPENROUTER_API_KEY obrigatórios")
	}
	baseURL := "https://openrouter.ai"
	client, err := httpclient.NewClient(fake.NewProvider(),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget("openrouter"),
		httpclient.WithTimeout(45*time.Second),
	)
	require.NoError(t, err)
	return llm.NewOpenRouterProvider(client, llm.Config{
		Model:          "openai/gpt-4o-mini",
		BaseURL:        baseURL,
		APIKey:         apiKey,
		HTTPReferer:    "https://github.com/LimaTeixeiraTecnologia/mecontrola",
		XTitle:         "mecontrola-tools-harness",
		MaxTokens:      1536,
		Temperature:    0,
		RequestTimeout: 45 * time.Second,
	}, fake.NewProvider())
}

type captureResult struct {
	called bool
	name   string
	args   map[string]any
}

func buildCaptureTool(name, description string, schema map[string]any, capture **captureResult) tool.ToolHandle {
	in := llm.Schema{
		Name:   name + "_input",
		Strict: false,
		Schema: schema,
	}
	out := llm.Schema{
		Name:   name + "_output",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"ok": map[string]any{"type": "boolean"}},
			"required":             []string{"ok"},
			"additionalProperties": false,
		},
	}
	type output struct {
		OK bool `json:"ok"`
	}
	return tool.NewTool[map[string]any, output](name, description, in, out,
		func(_ context.Context, in map[string]any) (output, error) {
			*capture = &captureResult{called: true, name: name, args: in}
			return output{OK: true}, nil
		},
	)
}

type harnessScenario struct {
	input         string
	expectedTool  string
	expectedTools []string
	tools         []tool.ToolHandle
}

type allToolsScorer struct {
	id    string
	tools []string
}

func (s *allToolsScorer) ID() string              { return s.id }
func (s *allToolsScorer) Kind() scorer.ScorerKind { return scorer.ScorerKindCodeBased }

func (s *allToolsScorer) Score(_ context.Context, sample scorer.RunSample) (scorer.ScoreResult, error) {
	called := make(map[string]bool, len(sample.ToolCalls))
	for _, tc := range sample.ToolCalls {
		called[tc.Name] = true
	}
	for _, t := range s.tools {
		if !called[t] {
			calledNames := make([]string, 0, len(sample.ToolCalls))
			for _, tc := range sample.ToolCalls {
				calledNames = append(calledNames, tc.Name)
			}
			return scorer.ScoreResult{
				Score:    0.0,
				Reason:   fmt.Sprintf("tool esperada %q não foi chamada; chamadas: %v", t, calledNames),
				Metadata: map[string]any{"missing": t, "called": calledNames},
			}, nil
		}
	}
	return scorer.ScoreResult{
		Score:    1.0,
		Reason:   fmt.Sprintf("todas as tools esperadas foram chamadas: %v", s.tools),
		Metadata: map[string]any{"tools": s.tools},
	}, nil
}

func newAllToolsScorer(tools []string) scorer.Scorer {
	return &allToolsScorer{
		id:    "all-tools:" + strings.Join(tools, "+"),
		tools: tools,
	}
}

func isTransientLLMError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, marker := range []string{"GOAWAY", "unexpected EOF", "connection reset", "EOF", "timeout", "temporarily", "503", "502", "429"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func runHarnessScenario(t *testing.T, ctx context.Context, provider llm.Provider, s harnessScenario) scorer.ScoreResult {
	t.Helper()
	obs := fake.NewProvider()
	a := agents.BuildMeControlaAgent(provider, s.tools, nil, obs)

	var result agent.Result
	var err error
	for attempt := 1; attempt <= 4; attempt++ {
		result, err = a.Execute(ctx, agent.Request{
			AgentID: agents.MecontrolaAgentID,
			Messages: []llm.Message{
				{Role: "user", Content: s.input},
			},
			MaxTokens: 512,
		})
		if err == nil || !isTransientLLMError(err) {
			break
		}
		t.Logf("[%s] tentativa %d falhou por erro transitório de transporte: %v", s.expectedTool, attempt, err)
		time.Sleep(time.Duration(attempt) * 2 * time.Second)
	}
	require.NoError(t, err)

	sample := scorer.RunSample{Input: s.input, Output: result.Content}
	for _, tc := range result.ToolCalls {
		sample.ToolCalls = append(sample.ToolCalls, scorer.ToolCallRecord{
			ID:   fmt.Sprintf("%d", len(sample.ToolCalls)),
			Name: tc.Tool,
		})
	}

	var sc scorer.Scorer
	label := s.expectedTool
	if len(s.expectedTools) > 0 {
		sc = newAllToolsScorer(s.expectedTools)
		label = strings.Join(s.expectedTools, "+")
	} else {
		sc = NewExpectedToolScorer(s.expectedTool)
	}
	scored, err := sc.Score(ctx, sample)
	require.NoError(t, err)
	t.Logf("[%s] score=%.2f reason=%s tool_calls=%v", label, scored.Score, scored.Reason, result.ToolCalls)
	return scored
}

func buildAllFakeTools(userID string) ([]tool.ToolHandle, map[string]**captureResult) {
	captures := make(map[string]**captureResult)

	makeCapture := func(name string) **captureResult {
		var c *captureResult
		captures[name] = &c
		return &c
	}

	baseSchema := func(fields ...string) map[string]any {
		props := map[string]any{}
		for _, f := range fields {
			props[f] = map[string]any{"type": "string"}
		}
		return map[string]any{
			"type":                 "object",
			"properties":           props,
			"additionalProperties": false,
		}
	}

	tools := []tool.ToolHandle{
		buildCaptureTool("register_expense", "Registra uma despesa", baseSchema("description", "amountCents", "paymentMethod", "occurredAt", "categoryId"), makeCapture("register_expense")),
		buildCaptureTool("register_income", "Registra uma receita", baseSchema("description", "amountCents", "occurredAt"), makeCapture("register_income")),
		buildCaptureTool("create_recurrence", "Cria template recorrente", baseSchema("description", "amountCents", "frequency", "dayOfMonth", "direction"), makeCapture("create_recurrence")),
		buildCaptureTool("query_month", "Consulta o resumo e os lançamentos do mês financeiro do usuário; use para 'quanto gastei?', 'como foi meu mês?', 'última transação' e 'últimas N transações'.", baseSchema("refMonth"),
			func() **captureResult {
				var c *captureResult
				captures["query_month"] = &c
				captureTool := func(_ context.Context, in map[string]any) (map[string]any, error) {
					c = &captureResult{called: true, name: "query_month", args: in}
					return map[string]any{
						"refMonth":     "2026-07",
						"incomeCents":  500000,
						"outcomeCents": 320000,
						"totalCents":   180000,
						"entries":      []any{},
					}, nil
				}
				_ = captureTool
				return &c
			}()),
		buildCaptureTool("get_transaction", "Retorna os detalhes de um lançamento de transação pelo ID.", baseSchema("txId"), makeCapture("get_transaction")),
		buildCaptureTool("search_transactions", "Pesquisa lançamentos do usuário por termo explícito no mês informado; use APENAS quando o usuário fornecer uma palavra-chave ou termo de busca específico — NÃO usar para 'última transação' ou 'últimas N transações'.", baseSchema("query", "refMonth"), makeCapture("search_transactions")),
		buildCaptureTool("list_cards", "Lista os cartões cadastrados do usuário; use apenas quando o usuário pedir explicitamente para ver, listar ou saber quais são seus cartões, nunca como etapa preparatória de um registro de compra ou consulta de fatura", baseSchema(),
			func() **captureResult {
				var c *captureResult
				captures["list_cards"] = &c
				return &c
			}()),
		buildCaptureTool("get_card", "Busca dados de um cartão pelo ID", baseSchema("cardId"), makeCapture("get_card")),
		buildCaptureTool("count_cards", "Conta cartões do usuário", baseSchema(), makeCapture("count_cards")),
		buildCaptureTool("best_purchase_day", "Calcula melhor dia de compra", baseSchema("bank", "dueDay"), makeCapture("best_purchase_day")),
		buildCaptureTool("query_card_invoice", "Consulta fatura do cartão", baseSchema("cardId", "refMonth"), makeCapture("query_card_invoice")),
		buildCaptureTool("list_recurrences", "Lista templates de recorrência", baseSchema(), makeCapture("list_recurrences")),
		buildCaptureTool("update_recurrence", "Solicita atualização de recorrência", baseSchema("recurrenceId", "description"), makeCapture("update_recurrence")),
		buildCaptureTool("delete_recurrence", "Solicita exclusão de recorrência", baseSchema("recurrenceId"), makeCapture("delete_recurrence")),
		buildCaptureTool("list_categories", "Lista categorias disponíveis", baseSchema(), makeCapture("list_categories")),
		buildCaptureTool("classify_category", "Classifica lançamento por categoria", baseSchema("description", "direction"), makeCapture("classify_category")),
		buildCaptureTool("query_plan", "Consulta o plano orçamentário mensal e alertas do usuário; use para 'como está meu orçamento?', 'panorama do mês' (junto com query_month) e 'orçamento completo'.",
			baseSchema("refMonth"),
			func() **captureResult {
				var c *captureResult
				captures["query_plan"] = &c
				return &c
			}()),
		buildCaptureTool("adjust_allocation", "Ajusta alocação de categoria", baseSchema("rootSlug", "basisPoints"), makeCapture("adjust_allocation")),
		buildCaptureTool("suggest_allocation", "Sugere distribuição de alocação", baseSchema("totalCents"), makeCapture("suggest_allocation")),
		buildCaptureTool("edit_entry", "Inicia a edição de um lançamento pelo ID; chame imediatamente quando o usuário disser que quer editar um lançamento identificado, mesmo sem saber ainda o que mudar, pois a própria ferramenta retorna a confirmação necessária", baseSchema("entryId", "entryKind"), makeCapture("edit_entry")),
		buildCaptureTool("delete_entry", "Solicita exclusão de lançamento ou cartão", baseSchema("targetRef", "targetKind"), makeCapture("delete_entry")),
		buildCaptureTool("update_card", "Solicita atualização de cartão", baseSchema("cardId", "nickname", "dueDay"), makeCapture("update_card")),
		buildCaptureTool("resolve_card", "Resolve o cartão de crédito do usuário pelo apelido informado, retornando o cardId; use como etapa obrigatória antes de registrar compra no crédito OU antes de consultar a fatura do cartão (query_card_invoice).", baseSchema("nickname"), makeCapture("resolve_card")),
	}

	_ = userID
	return tools, captures
}

func TestRealLLM_ToolCoverage_All22Tools(t *testing.T) {
	provider := buildHarnessProvider(t)
	userID := uuid.New().String()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	tools, _ := buildAllFakeTools(userID)

	scenarios := []harnessScenario{
		{input: "gastei 50 reais no almoço hoje, pagamento débito", expectedTool: "register_expense", tools: tools},
		{input: "recebi meu salário de 5000 reais hoje", expectedTool: "register_income", tools: tools},
		{input: "quero criar um lançamento recorrente de academia 150 reais todo dia 10", expectedTool: "create_recurrence", tools: tools},
		{input: "quanto gastei esse mês?", expectedTool: "query_month", tools: tools},
		{input: "busca o lançamento com id abc-123", expectedTool: "get_transaction", tools: tools},
		{input: "procura lançamentos com a palavra mercado", expectedTool: "search_transactions", tools: tools},
		{input: "quais são meus cartões?", expectedTool: "list_cards", tools: tools},
		{input: "me mostra os dados do cartão id cartao-001", expectedTool: "get_card", tools: tools},
		{input: "quantos cartões eu tenho?", expectedTool: "count_cards", tools: tools},
		{input: "qual é o melhor dia para comprar no banco nubank com vencimento dia 10?", expectedTool: "best_purchase_day", tools: tools},
		{input: "qual é a fatura do meu cartão cartao-001 em julho 2026?", expectedTool: "query_card_invoice", tools: tools},
		{input: "lista minhas recorrências", expectedTool: "list_recurrences", tools: tools},
		{input: "quero atualizar a descrição da recorrência rec-001 para plano de saúde", expectedTool: "update_recurrence", tools: tools},
		{input: "quero excluir a recorrência rec-002", expectedTool: "delete_recurrence", tools: tools},
		{input: "quais categorias existem?", expectedTool: "list_categories", tools: tools},
		{input: "classifica o lançamento supermercado como categoria de alimentação", expectedTool: "classify_category", tools: tools},
		{input: "como está meu orçamento deste mês?", expectedTool: "query_plan", tools: tools},
		{input: "ajusta a alocação da categoria custo_fixo para 35 porcento", expectedTool: "adjust_allocation", tools: tools},
		{input: "me sugere como distribuir 8000 reais no orçamento", expectedTool: "suggest_allocation", tools: tools},
		{input: "quero editar o lançamento id lanc-001", expectedTool: "edit_entry", tools: tools},
		{input: "quero excluir o lançamento id lanc-002", expectedTool: "delete_entry", tools: tools},
		{input: "quero atualizar o apelido do cartão id cartao-001 para nubank pessoal", expectedTool: "update_card", tools: tools},
		{input: "como estou indo?", expectedTools: []string{"query_month", "query_plan"}, tools: tools},
		{input: "como foi meu orçamento de janeiro/2026?", expectedTool: "query_plan", tools: tools},
		{input: "como está meu orçamento do mês atual?", expectedTool: "query_plan", tools: tools},
		{input: "quanto está minha fatura do cartão nubank?", expectedTool: "resolve_card", tools: tools},
		{input: "qual foi a minha última transação?", expectedTool: "query_month", tools: tools},
		{input: "quais foram as minhas últimas 5 transações?", expectedTool: "query_month", tools: tools},
		{input: "me mostra o orçamento completo", expectedTool: "query_plan", tools: tools},
	}

	require.Len(t, scenarios, 29, "harness deve cobrir 22 tools existentes + 7 cenários C1-C7")

	hits := 0
	uncovered := []string{}
	for _, s := range scenarios {
		result := runHarnessScenario(t, ctx, provider, s)
		label := s.expectedTool
		if len(s.expectedTools) > 0 {
			label = strings.Join(s.expectedTools, "+")
		}
		if result.Score >= 1.0 {
			hits++
		} else {
			uncovered = append(uncovered, label)
		}
	}

	m04 := float64(hits) / float64(len(scenarios))
	t.Logf("M-04 (acerto de tool esperada): %.2f (%d/%d)", m04, hits, len(scenarios))
	t.Logf("Tools não exercidas ou erradas: %v", uncovered)

	require.Empty(t, uncovered, "RF-29: todas as tools e cenários C1-C7 devem ser exercidos")
	require.GreaterOrEqual(t, m04, 0.90, "M-04: acerto de tool esperada deve ser >= 0.90")
}

func TestRealLLM_EP01_AntiSimulation_RegisterExpenseDoesNotFakeSuccess(t *testing.T) {
	provider := buildHarnessProvider(t)
	obs := fake.NewProvider()

	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: false,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"description": map[string]any{"type": "string"}, "amountCents": map[string]any{"type": "integer"}},
			"additionalProperties": false,
		},
	}
	failingTool := tool.NewTool[map[string]any, map[string]any]("register_expense", "Registra despesa", in, llm.Schema{},
		func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return nil, fmt.Errorf("EP-01: falha de persistência simulada")
		},
	)

	a := agents.BuildMeControlaAgent(provider, []tool.ToolHandle{failingTool}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: agents.MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "gastei 30 reais no café, débito"},
		},
		MaxTokens: 512,
	})
	require.NoError(t, err)
	t.Logf("EP-01 resposta: %s", result.Content)

	positivas := []string{"registrei com sucesso", "foi registrado", "lançamento confirmado", "registrado com sucesso"}
	for _, p := range positivas {
		require.NotContainsf(t, result.Content, p, "EP-01: agente não deve afirmar sucesso quando tool falhou (frase: %q)", p)
	}
}

func TestRealLLM_EP05_RunPersistsRoleTool(t *testing.T) {
	provider := buildHarnessProvider(t)
	obs := fake.NewProvider()

	var toolCallCount int
	in := llm.Schema{
		Name:   "query_month_input",
		Strict: false,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"refMonth": map[string]any{"type": "string"}},
			"additionalProperties": false,
		},
	}
	queryTool := tool.NewTool[map[string]any, map[string]any]("query_month", "Consulta resumo do mês", in, llm.Schema{},
		func(_ context.Context, _ map[string]any) (map[string]any, error) {
			toolCallCount++
			return map[string]any{
				"refMonth":     "2026-07",
				"incomeCents":  500000,
				"outcomeCents": 320000,
				"totalCents":   180000,
				"entries":      []any{},
			}, nil
		},
	)

	a := agents.BuildMeControlaAgent(provider, []tool.ToolHandle{queryTool}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: agents.MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "quanto gastei esse mês?"},
		},
		MaxTokens: 512,
	})
	require.NoError(t, err)
	require.Greater(t, toolCallCount, 0, "EP-05: query_month deve ter sido chamada pelo agente")
	t.Logf("EP-05 resposta: %s tool_calls=%d", result.Content, toolCallCount)
}
