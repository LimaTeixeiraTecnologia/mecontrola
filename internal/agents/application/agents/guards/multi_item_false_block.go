package guards

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const multiItemFalseBlockReask = "Me conta de novo esse lançamento, o valor e onde foi, que eu registro pra você. 🙂"

const multiItemFalseBlockCorrectionReask = "Entendi que você quer corrigir um lançamento. Me confirma qual é e o que mudou (ex.: \"o uber de 30 virou 35\")? 🙂"

var multiItemFalseBlockMarkers = []string{
	"mais de um lançamento",
	"um de cada vez",
	"um lançamento por vez",
	"um por vez",
}

type multiItemFalseBlockGuard struct{}

func NewMultiItemFalseBlockGuard() PostGuard {
	return &multiItemFalseBlockGuard{}
}

func (g *multiItemFalseBlockGuard) Name() string {
	return "multi_item_false_block"
}

func (g *multiItemFalseBlockGuard) Inspect(_ context.Context, in agent.Request, out agent.Result) GuardDecision {
	if !containsMultiItemBlockMarker(out.Content) {
		return GuardDecision{}
	}
	message := lastUserMessageContent(in.Messages)
	if DetectMultipleMonetaryValues(message) && !IsMultiItemExempt(message) {
		return GuardDecision{}
	}
	if hasSuccessfulWriteTool(out.ToolCalls) {
		return GuardDecision{}
	}
	reask := multiItemFalseBlockReask
	if IsCorrectionOrEditIntent(message) {
		reask = multiItemFalseBlockCorrectionReask
	}
	forced := out
	forced.Content = reask
	forced.ToolOutcome = agent.ToolOutcomeClarify
	return GuardDecision{Handled: true, Result: forced}
}

func containsMultiItemBlockMarker(content string) bool {
	lower := strings.ToLower(content)
	for _, marker := range multiItemFalseBlockMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
