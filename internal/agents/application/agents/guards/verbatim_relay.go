package guards

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

var verbatimFieldPriority = []string{
	"message",
	"impactNote",
	"clarifyPrompt",
	"confirmationPrompt",
	"offerCreatePrompt",
}

type verbatimRelayGuard struct{}

func NewVerbatimRelayGuard() PostGuard {
	return &verbatimRelayGuard{}
}

func (g *verbatimRelayGuard) Name() string {
	return "verbatim_relay"
}

func (g *verbatimRelayGuard) Inspect(_ context.Context, _ agent.Request, out agent.Result) GuardDecision {
	expected := expectedVerbatimText(out.ToolCalls)
	if expected == "" || expected == out.Content {
		return GuardDecision{}
	}
	forced := out
	forced.Content = expected
	return GuardDecision{Handled: true, Result: forced}
}

func expectedVerbatimText(calls []agent.ToolCallRecord) string {
	for _, call := range slices.Backward(calls) {
		if call.Outcome != agent.ToolCallOutcomeSuccess {
			continue
		}
		text := extractVerbatimField(call.Content)
		if text != "" {
			return text
		}
	}
	return ""
}

func extractVerbatimField(rawJSON string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &payload); err != nil {
		return ""
	}
	for _, field := range verbatimFieldPriority {
		value, ok := payload[field]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if ok && text != "" {
			return text
		}
	}
	return ""
}
