package guards

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

var categoryWithoutToolMarkers = []string{
	"em qual categoria isso se encaixa?",
	"qual subcategoria?",
	"qual se encaixa melhor?",
	"responda o número ou o nome.",
	"ou me diga o nome da subcategoria.",
}

type categoryWithoutToolGuard struct{}

func NewCategoryWithoutToolGuard() PostGuard {
	return &categoryWithoutToolGuard{}
}

func (g *categoryWithoutToolGuard) Name() string {
	return "category_without_tool"
}

func (g *categoryWithoutToolGuard) Inspect(_ context.Context, _ agent.Request, out agent.Result) GuardDecision {
	if ExpectedVerbatimText(out.ToolCalls) != "" {
		return GuardDecision{}
	}
	if !containsCategoryMarker(out.Content) {
		return GuardDecision{}
	}
	forced := out
	forced.Content = successWithoutToolFallbackMessage
	forced.ToolOutcome = agent.ToolOutcomeUsecaseError
	forced.FailureReason = "guard category_without_tool: pergunta de categoria fabricada pelo LLM sem tool call"
	return GuardDecision{Handled: true, Result: forced}
}

func containsCategoryMarker(content string) bool {
	lower := strings.ToLower(content)
	for _, marker := range categoryWithoutToolMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
