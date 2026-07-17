package guards

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const multiItemFalseBlockReask = "Me conta de novo esse lançamento — o valor e onde foi — que eu registro pra você. 🙂"

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
	if DetectMultipleMonetaryValues(lastUserMessageContent(in.Messages)) {
		return GuardDecision{}
	}
	if hasSuccessfulWriteTool(out.ToolCalls) {
		return GuardDecision{}
	}
	forced := out
	forced.Content = multiItemFalseBlockReask
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
