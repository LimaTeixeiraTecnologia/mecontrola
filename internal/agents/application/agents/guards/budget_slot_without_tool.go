package guards

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

var (
	budgetSlotWithoutToolQuestionMarkers = []string{
		"distribuição do seu orçamento",
		"distribuicao do seu orcamento",
		"distribuição do orçamento",
		"distribuicao do orcamento",
		"distribuir o seu orçamento",
		"distribuir o seu orcamento",
		"distribuir seu orçamento",
		"distribuir seu orcamento",
		"valor total do seu orçamento",
		"valor total do seu orcamento",
		"valor total que você gostaria",
		"valor total que voce gostaria",
		"valor total para o novo orçamento",
		"valor total para o novo orcamento",
		"entre as categorias",
	}

	budgetSlotWithoutToolTools = map[string]struct{}{
		"create_budget":     {},
		"edit_budget_total": {},
		"adjust_allocation": {},
	}
)

type budgetSlotWithoutToolGuard struct{}

func NewBudgetSlotWithoutToolGuard() PostGuard {
	return &budgetSlotWithoutToolGuard{}
}

func (g *budgetSlotWithoutToolGuard) Name() string {
	return "budget_slot_without_tool"
}

func (g *budgetSlotWithoutToolGuard) Inspect(_ context.Context, _ agent.Request, out agent.Result) GuardDecision {
	if ExpectedVerbatimText(out.ToolCalls) != "" {
		return GuardDecision{}
	}
	if hasBudgetSlotTool(out.ToolCalls) {
		return GuardDecision{}
	}
	if !containsBudgetSlotMarker(out.Content) {
		return GuardDecision{}
	}
	forced := out
	forced.Content = successWithoutToolFallbackMessage
	forced.ToolOutcome = agent.ToolOutcomeUsecaseError
	forced.FailureReason = "guard budget_slot_without_tool: pergunta de slot de orçamento fabricada pelo LLM sem tool call"
	return GuardDecision{Handled: true, Retryable: true, Result: forced}
}

func hasBudgetSlotTool(calls []agent.ToolCallRecord) bool {
	for _, call := range calls {
		if _, ok := budgetSlotWithoutToolTools[call.Tool]; ok {
			return true
		}
	}
	return false
}

func containsBudgetSlotMarker(content string) bool {
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "orçamento") && !strings.Contains(lower, "orcamento") {
		return false
	}
	if !strings.Contains(lower, "?") {
		return false
	}
	for _, marker := range budgetSlotWithoutToolQuestionMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
