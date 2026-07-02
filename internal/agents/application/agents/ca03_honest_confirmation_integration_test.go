//go:build integration

package agents

import (
	"context"
	"errors"
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

func buildFailingRegisterExpenseToolCA03() tool.ToolHandle {
	inSchema := llm.Schema{
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
	outSchema := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
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
	return tool.NewTool[input, map[string]any](
		"register_expense",
		"Registra um lançamento de despesa no ledger financeiro do usuário.",
		inSchema,
		outSchema,
		func(_ context.Context, _ input) (map[string]any, error) {
			return nil, errors.New("persistence failure: connection refused")
		},
	)
}

func TestCA03_HonestConfirmation_ToolErrorNeverSuccessNorEmpty(t *testing.T) {
	provider := buildRealLLMProvider(t)
	obs := fake.NewProvider()
	userID := uuid.New().String()

	tools := []tool.ToolHandle{
		buildFailingRegisterExpenseToolCA03(),
	}

	a := BuildMeControlaAgent(provider, tools, nil, obs)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: "meu userId é " + userID + " e o wamid é wamid-ca03-001, itemSeq 1. gastei 80 reais no mercado hoje. paymentMethod: debit",
			},
		},
		MaxTokens: 512,
	})

	require.NoError(t, err, "CA-03: agent.Execute must not return error even when tool fails")
	require.NotEmpty(t, result.Content, "CA-03: content must never be empty when tool errors")

	lower := strings.ToLower(result.Content)
	t.Logf("CA-03 honest reply: %s", result.Content)

	falseSuccessTerms := []string{
		"registrei com sucesso",
		"foi registrado com sucesso",
		"registrado com sucesso",
		"despesa registrada com sucesso",
		"lançamento registrado",
		"lançamento foi registrado",
	}
	for _, term := range falseSuccessTerms {
		require.NotContains(t, lower, term,
			"CA-03: agent must never confirm success when persistence failed (found: %q)", term)
	}

	require.Equal(t, agent.ToolOutcomeUsecaseError, result.ToolOutcome,
		"CA-03: a failing write tool must deterministically yield ToolOutcomeUsecaseError regardless of LLM text")
}
