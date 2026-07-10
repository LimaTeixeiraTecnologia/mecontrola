package guards

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const cardProvenanceFallbackMessage = "Antes de continuar, preciso saber qual cartão você quer usar. Pode me dizer o apelido dele (ex.: nubank)?"

var cardProvenanceResolverTools = map[string]struct{}{
	"resolve_card": {},
	"list_cards":   {},
}

var cardProvenanceConsumerTools = map[string]struct{}{
	"register_expense":   {},
	"create_recurrence":  {},
	"query_card_invoice": {},
}

type cardProvenanceGuard struct{}

func NewCardProvenanceGuard() PostGuard {
	return &cardProvenanceGuard{}
}

func (g *cardProvenanceGuard) Name() string {
	return "card_provenance"
}

func (g *cardProvenanceGuard) Inspect(_ context.Context, _ agent.Request, out agent.Result) GuardDecision {
	if consumerWithoutPriorResolution(out.ToolCalls) {
		forced := out
		forced.Content = cardProvenanceFallbackMessage
		forced.ToolOutcome = agent.ToolOutcomeClarify
		return GuardDecision{Handled: true, Result: forced}
	}
	return GuardDecision{}
}

func consumerWithoutPriorResolution(calls []agent.ToolCallRecord) bool {
	resolved := false
	for _, call := range calls {
		if _, ok := cardProvenanceResolverTools[call.Tool]; ok {
			resolved = true
			continue
		}
		if _, ok := cardProvenanceConsumerTools[call.Tool]; ok && !resolved {
			return true
		}
	}
	return false
}
