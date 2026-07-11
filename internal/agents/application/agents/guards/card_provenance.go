package guards

import (
	"context"
	"encoding/json"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const cardProvenanceFallbackMessage = "Antes de continuar, preciso saber qual 💳 você quer usar. Pode me dizer o apelido dele (ex.: nubank)?"

var cardProvenanceResolverTools = map[string]struct{}{
	"resolve_card":           {},
	"resolve_card_not_found": {},
	"list_cards":             {},
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
	if cardResolutionNotFound(out.ToolCalls) {
		forced := out
		forced.Content = cardProvenanceFallbackMessage
		forced.ToolOutcome = agent.ToolOutcomeClarify
		return GuardDecision{Handled: true, Result: forced}
	}
	if consumerWithoutPriorResolution(out.ToolCalls) {
		forced := out
		forced.Content = cardProvenanceFallbackMessage
		forced.ToolOutcome = agent.ToolOutcomeClarify
		return GuardDecision{Handled: true, Result: forced}
	}
	return GuardDecision{}
}

func cardResolutionNotFound(calls []agent.ToolCallRecord) bool {
	for _, call := range calls {
		if call.Tool != "resolve_card" && call.Tool != "resolve_card_not_found" {
			continue
		}
		if foundValue, ok := call.ArgumentsJSON["found"].(bool); ok && !foundValue {
			return true
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(call.Content), &payload); err != nil {
			continue
		}
		foundValue, ok := payload["found"].(bool)
		if ok && !foundValue {
			return true
		}
	}
	return false
}

func consumerWithoutPriorResolution(calls []agent.ToolCallRecord) bool {
	resolved := false
	for _, call := range calls {
		if _, ok := cardProvenanceResolverTools[call.Tool]; ok {
			resolved = true
			continue
		}
		if _, ok := cardProvenanceConsumerTools[call.Tool]; ok && !resolved {
			return needsCardForConsumer(call.Tool, call.ArgumentsJSON)
		}
	}
	return false
}

func needsCardForConsumer(tool string, args map[string]any) bool {
	switch tool {
	case "query_card_invoice":
		return true
	case "register_expense", "create_recurrence":
		paymentMethod := extractPaymentMethod(args)
		return paymentMethod == "credit_card"
	default:
		return false
	}
}

func extractPaymentMethod(args map[string]any) string {
	if args == nil {
		return ""
	}
	raw, ok := args["paymentMethod"]
	if !ok {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		var s string
		if err := json.Unmarshal(mustMarshal(raw), &s); err == nil {
			return s
		}
		return ""
	}
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
