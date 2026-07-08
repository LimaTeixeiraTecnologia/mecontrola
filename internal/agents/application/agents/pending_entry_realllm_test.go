//go:build integration

package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

func buildPendingClarifyTool() tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":     map[string]any{"type": "integer"},
				"description":     map[string]any{"type": "string"},
				"paymentMethod":   map[string]any{"type": "string"},
				"occurredAt":      map[string]any{"type": "string"},
				"categoryId":      map[string]any{"type": "string"},
				"subcategoryId":   map[string]any{"type": "string"},
				"categoryVersion": map[string]any{"type": "integer"},
				"cardId":          map[string]any{"type": "string"},
				"installments":    map[string]any{"type": "integer"},
			},
			"required":             []string{"amountCents", "description", "paymentMethod"},
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
				"outcome":    map[string]any{"type": "string"},
			},
			"required":             []string{"resourceId", "kind", "isReplay", "outcome"},
			"additionalProperties": false,
		},
	}
	type input struct {
		AmountCents   int64  `json:"amountCents"`
		Description   string `json:"description"`
		PaymentMethod string `json:"paymentMethod"`
	}
	type output struct {
		ResourceID string `json:"resourceId"`
		Kind       string `json:"kind"`
		IsReplay   bool   `json:"isReplay"`
		Outcome    string `json:"outcome"`
	}
	return tool.NewTool[input, output]("register_expense", "Registra um lançamento de despesa no ledger financeiro do usuário.", in, out,
		func(_ context.Context, _ input) (output, error) {
			return output{
				ResourceID: "",
				Kind:       "pending",
				IsReplay:   false,
				Outcome:    "clarify",
			}, nil
		},
	)
}

func buildPendingSuccessTool() tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":     map[string]any{"type": "integer"},
				"description":     map[string]any{"type": "string"},
				"paymentMethod":   map[string]any{"type": "string"},
				"occurredAt":      map[string]any{"type": "string"},
				"categoryId":      map[string]any{"type": "string"},
				"subcategoryId":   map[string]any{"type": "string"},
				"categoryVersion": map[string]any{"type": "integer"},
				"cardId":          map[string]any{"type": "string"},
				"installments":    map[string]any{"type": "integer"},
			},
			"required":             []string{"amountCents", "description", "paymentMethod"},
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
				"outcome":    map[string]any{"type": "string"},
			},
			"required":             []string{"resourceId", "kind", "isReplay", "outcome"},
			"additionalProperties": false,
		},
	}
	type input struct {
		AmountCents   int64  `json:"amountCents"`
		Description   string `json:"description"`
		PaymentMethod string `json:"paymentMethod"`
	}
	type output struct {
		ResourceID string `json:"resourceId"`
		Kind       string `json:"kind"`
		IsReplay   bool   `json:"isReplay"`
		Outcome    string `json:"outcome"`
	}
	return tool.NewTool[input, output]("register_expense", "Registra um lançamento de despesa no ledger financeiro do usuário.", in, out,
		func(_ context.Context, _ input) (output, error) {
			return output{
				ResourceID: uuid.New().String(),
				Kind:       "transaction",
				IsReplay:   false,
				Outcome:    "routed",
			}, nil
		},
	)
}

func TestRealLLM_PendingEntry_CA01_ClarifyAsksOneQuestion(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 150,00 no mercado hoje no pix"},
		},
		MaxTokens: 256,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("CA-01 resposta: %s", result.Content)

	lower := strings.ToLower(result.Content)

	questionMarks := strings.Count(result.Content, "?")
	require.LessOrEqual(t, questionMarks, 1, "CA-01: agente deve fazer no maximo uma pergunta")

	falseSucessTerms := []string{"registrei", "anotei", "salvo", "foi registrada", "registrada com sucesso"}
	for _, term := range falseSucessTerms {
		require.NotContains(t, lower, term, "CA-01 M-03=0: agente nao deve confirmar sucesso sem write real")
	}

	require.NotContains(t, lower, "r$ 150", "CA-01 M-02: agente nao deve repetir valor no slot de categoria")
	require.NotContains(t, lower, "pix", "CA-01 M-02: agente nao deve repetir pagamento no slot de categoria")
}

func TestRealLLM_PendingEntry_CA06_LedgerError_HonestResponse(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{buildFailingRegisterExpenseTool()}, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 80,00 na farmácia hoje no pix"},
		},
		MaxTokens: 256,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("CA-06 G7-15 resposta: %s", result.Content)

	lower := strings.ToLower(result.Content)
	falseSucessTerms := []string{"registrei", "registrada com sucesso", "anotei", "foi registrado"}
	for _, term := range falseSucessTerms {
		require.NotContains(t, lower, term, "CA-06 M-03=0: agente nao deve confirmar sucesso com erro de ledger")
	}
}

func TestRealLLM_PendingEntry_NoInfraInResponse(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 200,00 no supermercado ontem no débito"},
		},
		MaxTokens: 256,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("NoInfra resposta: %s", result.Content)

	lower := strings.ToLower(result.Content)
	infraTerms := []string{"workflow", "pendência interna", "correlação", "thread_id", "resource_id", "correlation_key"}
	for _, term := range infraTerms {
		require.NotContains(t, lower, term, "agente nao deve mencionar infraestrutura ao usuario")
	}
}

func TestRealLLM_PendingEntry_CA12_DoubleAsteriskProibido(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 320,00 no supermercado hoje no pix"},
		},
		MaxTokens: 256,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("CA-12 WhatsApp formatting resposta: %s", result.Content)

	require.NotContains(t, result.Content, "**", "CA-12: agente nao deve usar duplo asterisco (WhatsApp incompativel)")
}

func TestRealLLM_PendingEntry_CA04_MultipleCandidates_ListaLegivel(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	in := llm.Schema{
		Name:   "classify_category_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"term": map[string]any{"type": "string"},
				"kind": map[string]any{"type": "string"},
			},
			"required":             []string{"term", "kind"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "classify_category_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"writeDecision": map[string]any{"type": "string"},
				"categoryId":    map[string]any{"type": "string"},
				"subcategoryId": map[string]any{"type": "string"},
				"path":          map[string]any{"type": "string"},
				"candidates":    map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			},
			"required":             []string{"writeDecision"},
			"additionalProperties": false,
		},
	}
	type classifyInput struct {
		Term string `json:"term"`
		Kind string `json:"kind"`
	}
	type candidate struct {
		Path string `json:"path"`
	}
	type classifyOutput struct {
		WriteDecision string      `json:"writeDecision"`
		CategoryID    string      `json:"categoryId"`
		SubcategoryID string      `json:"subcategoryId"`
		Path          string      `json:"path"`
		Candidates    []candidate `json:"candidates"`
	}
	classifyTool := tool.NewTool[classifyInput, classifyOutput]("classify_category", "Classifica termo em categoria.", in, out,
		func(_ context.Context, _ classifyInput) (classifyOutput, error) {
			return classifyOutput{
				WriteDecision: "blocked",
				Candidates: []candidate{
					{Path: "Custo Fixo > Plano de Saúde"},
					{Path: "Custo Fixo > Consultas e Exames"},
					{Path: "Custo Fixo > Terapia e Saúde Mental"},
				},
			}, nil
		},
	)

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{buildPendingClarifyTool(), classifyTool}, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Paguei R$ 350,00 de plano de saúde hoje no boleto"},
			{Role: "assistant", Content: "Qual é a categoria para esse lançamento?"},
			{Role: "user", Content: "saúde"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("CA-04 multiplos candidatos resposta: %s", result.Content)

	require.NotContains(t, result.Content, "**", "CA-04: sem duplo asterisco")

	hasNumbers := strings.Contains(result.Content, "1.") || strings.Contains(result.Content, "1)")
	hasSaudeReference := strings.Contains(strings.ToLower(result.Content), "saúde") || strings.Contains(strings.ToLower(result.Content), "saude")
	require.True(t, hasNumbers || hasSaudeReference, "CA-04: agente deve mostrar opcoes de categoria de forma legivel")
}

func TestRealLLM_R2_DataRelativa_Ontem(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	type capturedInput struct {
		PaymentMethod string `json:"paymentMethod"`
		OccurredAt    string `json:"occurredAt"`
	}
	var captured capturedInput

	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"occurredAt":    map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description", "paymentMethod"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome"},
			"additionalProperties": false,
		},
	}

	handle := tool.NewTool[capturedInput, map[string]any]("register_expense", "Registra um lançamento de despesa.", in, out,
		func(_ context.Context, inp capturedInput) (map[string]any, error) {
			captured = inp
			return map[string]any{"outcome": "clarify"}, nil
		},
	)

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{handle}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Ontem fui na feira e gastei cinquenta reais em pastel no dinheiro"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("R2 resposta: %s | paymentMethod=%s occurredAt=%s", result.Content, captured.PaymentMethod, captured.OccurredAt)

	require.NotEmpty(t, captured.PaymentMethod, "R2 M-01: paymentMethod deve ser extraído")
	require.NotContains(t, result.Content, "**", "R2 RF-23: sem duplo asterisco")
}

func TestRealLLM_R5_Receita_DecimoTerceiro(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	var incomeToolCalled bool
	in := llm.Schema{
		Name:   "register_income_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents": map[string]any{"type": "integer"},
				"description": map[string]any{"type": "string"},
				"occurredAt":  map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_income_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome"},
			"additionalProperties": false,
		},
	}
	type incomeInput struct {
		AmountCents int64  `json:"amountCents"`
		Description string `json:"description"`
	}
	handle := tool.NewTool[incomeInput, map[string]any]("register_income", "Registra um lançamento de receita.", in, out,
		func(_ context.Context, _ incomeInput) (map[string]any, error) {
			incomeToolCalled = true
			return map[string]any{"outcome": "clarify"}, nil
		},
	)

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{handle}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Recebi meu décimo terceiro de R$ 10.000,00"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("R5 resposta: %s | incomeToolCalled=%v", result.Content, incomeToolCalled)

	require.True(t, incomeToolCalled, "R5 M-01: deve chamar register_income para receita")
	require.NotContains(t, result.Content, "**", "R5 RF-23: sem duplo asterisco")
}

func TestRealLLM_R6_Receita_Freelancer(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	var incomeToolCalled bool
	var capturedAmount int64
	in := llm.Schema{
		Name:   "register_income_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents": map[string]any{"type": "integer"},
				"description": map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_income_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome"},
			"additionalProperties": false,
		},
	}
	type incomeInput struct {
		AmountCents int64  `json:"amountCents"`
		Description string `json:"description"`
	}
	handle := tool.NewTool[incomeInput, map[string]any]("register_income", "Registra um lançamento de receita.", in, out,
		func(_ context.Context, inp incomeInput) (map[string]any, error) {
			incomeToolCalled = true
			capturedAmount = inp.AmountCents
			return map[string]any{"outcome": "clarify"}, nil
		},
	)

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{handle}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Recebi duzentos reais de um freelancer"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("R6 resposta: %s | incomeToolCalled=%v amount=%d", result.Content, incomeToolCalled, capturedAmount)

	require.True(t, incomeToolCalled, "R6 M-01: deve chamar register_income para receita variável")
	if capturedAmount > 0 {
		require.Equal(t, int64(20000), capturedAmount, "R6 M-01: valor deve ser R$ 200,00 (20000 centavos)")
	}
}

func TestRealLLM_R7_CategoriaIncerta_PedeEsclarecimento(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei 35 reais com algo pro trabalho"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("R7 resposta: %s", result.Content)

	require.NotContains(t, result.Content, "**", "R7 RF-23: sem duplo asterisco")

	lower := strings.ToLower(result.Content)
	falseSucessTerms := []string{"registrei", "anotei", "foi registrado", "registrado com sucesso"}
	for _, term := range falseSucessTerms {
		require.NotContains(t, lower, term, "R7 M-03=0: agente nao deve confirmar sucesso sem write real")
	}
}

func TestRealLLM_DiaDaSemana_Terca(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	var capturedOccurredAt string
	type expInput struct {
		AmountCents   int64  `json:"amountCents"`
		Description   string `json:"description"`
		PaymentMethod string `json:"paymentMethod"`
		OccurredAt    string `json:"occurredAt"`
	}
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"occurredAt":    map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description", "paymentMethod"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome"},
			"additionalProperties": false,
		},
	}
	handle := tool.NewTool[expInput, map[string]any]("register_expense", "Registra um lançamento de despesa.", in, out,
		func(_ context.Context, inp expInput) (map[string]any, error) {
			capturedOccurredAt = inp.OccurredAt
			return map[string]any{"outcome": "clarify"}, nil
		},
	)

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{handle}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Na terça fui no mercado e gastei R$ 80,00 no pix"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("DiasSemana terça resposta: %s | occurredAt=%s", result.Content, capturedOccurredAt)

	require.NotEmpty(t, capturedOccurredAt, "RF-07: agente deve repassar o texto de data (dia da semana) ao tool")
	require.NotContains(t, result.Content, "**", "RF-23: sem duplo asterisco")
}

func TestRealLLM_SemanaMesPassado_Rejeita(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Semana passada gastei R$ 120,00 no supermercado no pix"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("SemanaMesPassado resposta: %s", result.Content)

	lower := strings.ToLower(result.Content)
	falseSucessTerms := []string{"registrei", "anotei", "foi registrado", "registrado com sucesso"}
	for _, term := range falseSucessTerms {
		require.NotContains(t, lower, term, "RF-08 M-03=0: agente nao deve registrar com data vaga 'semana passada'")
	}
}

func TestRealLLM_MultiItem_RF16_UmPorVez(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	var toolCallCount int
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description", "paymentMethod"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome"},
			"additionalProperties": false,
		},
	}
	type expInput struct {
		AmountCents   int64  `json:"amountCents"`
		Description   string `json:"description"`
		PaymentMethod string `json:"paymentMethod"`
	}
	handle := tool.NewTool[expInput, map[string]any]("register_expense", "Registra um lançamento de despesa.", in, out,
		func(_ context.Context, _ expInput) (map[string]any, error) {
			toolCallCount++
			return map[string]any{"outcome": "clarify"}, nil
		},
	)

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{handle}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Hoje gastei 30 reais no ônibus e 15 no café"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("RF-16 multi-item resposta: %s | toolCalls=%d", result.Content, toolCallCount)

	lower := strings.ToLower(result.Content)
	falseSuccessTerms := []string{"registrei", "anotei", "foi registrado", "registrado com sucesso", "feito!", "pronto! seu gasto"}
	for _, term := range falseSuccessTerms {
		require.NotContains(t, lower, term, "RF-16 M-05=0: agente nao deve afirmar sucesso de escrita em mensagem multi-item")
	}
	require.NotContains(t, result.Content, "**", "RF-23: sem duplo asterisco")
	t.Logf("RF-16 M-05=0: nenhuma escrita persistida confirmada (tool retornou clarify em todas as %d chamadas)", toolCallCount)
}

func TestRealLLM_Cancelamento_NaoEscreve(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()

	a := BuildMeControlaAgent(provider, []tool.ToolHandle{buildPendingClarifyTool()}, nil, obs)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Gastei R$ 50,00 no mercado hoje no pix"},
			{Role: "assistant", Content: "Em qual categoria você enquadra esse gasto?"},
			{Role: "user", Content: "cancela"},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	t.Logf("cancelamento resposta: %s", result.Content)

	lower := strings.ToLower(result.Content)
	falseSucessTerms := []string{"registrei", "anotei", "foi registrado", "registrado com sucesso"}
	for _, term := range falseSucessTerms {
		require.NotContains(t, lower, term, "M-05=0: cancelamento nao deve resultar em escrita")
	}
}
