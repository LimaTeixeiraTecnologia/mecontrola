package guards

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

var (
	createCardDueDayRe = regexp.MustCompile(`(?i)\b(?:vencimento|vence)\s*(?:dia\s*)?([0-9]{1,2})\b`)
	createCardNameRe   = regexp.MustCompile(`(?i)(?:💳|cart[aã]o(?:\s+de\s+cr[eé]dito)?|card)\s+([^,]+?)(?:\s*,|\s+vencimento|\s+vence|$)`)
)

type createCardShortcutGuard struct {
	handle tool.ToolHandle
}

func NewCreateCardShortcutGuard(handle tool.ToolHandle) PreGuard {
	return &createCardShortcutGuard{handle: handle}
}

func (g *createCardShortcutGuard) Name() string {
	return "create_card_shortcut"
}

func (g *createCardShortcutGuard) Inspect(ctx context.Context, in agent.Request) GuardDecision {
	if g.handle == nil {
		return GuardDecision{}
	}
	args, ok := parseCreateCardShortcut(lastUserMessageContent(in.Messages), g.handle)
	if !ok {
		return GuardDecision{}
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return GuardDecision{}
	}
	raw, verbatim, err := g.handle.Invoke(ctx, argsJSON)
	if err != nil {
		return GuardDecision{}
	}
	content := createCardShortcutContent(raw, verbatim)
	return GuardDecision{
		Handled: true,
		Result: agent.Result{
			Content:     content,
			Mode:        agent.ExecutionModeSync,
			ToolOutcome: agent.ToolOutcomeClarify,
			ToolCalls: []agent.ToolCallRecord{{
				Tool:          g.handle.ID(),
				Outcome:       agent.ToolCallOutcomeSuccess,
				Content:       string(raw),
				ArgumentsJSON: args,
			}},
		},
	}
}

func parseCreateCardShortcut(message string, handle tool.ToolHandle) (map[string]any, bool) {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if !containsAnyText(normalized, "cadastr", "criar", "adicionar") {
		return nil, false
	}
	if !containsAnyText(normalized, "💳", "cartao", "cartão") {
		return nil, false
	}
	dueMatch := createCardDueDayRe.FindStringSubmatch(message)
	nameMatch := createCardNameRe.FindStringSubmatch(message)
	if len(dueMatch) != 2 || len(nameMatch) != 2 {
		return nil, false
	}
	dueDay, err := strconv.Atoi(dueMatch[1])
	if err != nil || dueDay < 1 || dueDay > 31 {
		return nil, false
	}
	name := strings.TrimSpace(nameMatch[1])
	name = strings.Trim(name, " .,;:")
	if name == "" {
		return nil, false
	}
	dueDayArg := any(dueDay)
	if toolPropertyWantsString(handle, "dueDay") {
		dueDayArg = strconv.Itoa(dueDay)
	}
	return map[string]any{
		"nickname": name,
		"bank":     name,
		"dueDay":   dueDayArg,
	}, true
}

func toolPropertyWantsString(handle tool.ToolHandle, property string) bool {
	if handle == nil {
		return false
	}
	params := handle.Parameters()
	props, ok := params["properties"].(map[string]any)
	if !ok {
		return false
	}
	definition, ok := props[property].(map[string]any)
	if !ok {
		return false
	}
	kind, _ := definition["type"].(string)
	return kind == "string"
}

func createCardShortcutContent(raw []byte, verbatim string) string {
	if strings.TrimSpace(verbatim) != "" {
		return verbatim
	}
	var payload struct {
		ConfirmationPrompt string `json:"confirmationPrompt"`
		ClarifyPrompt      string `json:"clarifyPrompt"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil {
		if strings.TrimSpace(payload.ConfirmationPrompt) != "" {
			return payload.ConfirmationPrompt
		}
		if strings.TrimSpace(payload.ClarifyPrompt) != "" {
			return payload.ClarifyPrompt
		}
	}
	return "Vou cadastrar esse 💳 e te pedir confirmação antes de concluir. ✅"
}
