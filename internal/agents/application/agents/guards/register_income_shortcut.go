package guards

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

var (
	incomeAmountRe      = regexp.MustCompile(`(?i)(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)\s*(?:reais|real)?`)
	incomeDescriptionRe = regexp.MustCompile(`(?i)\bde\s+([^0-9,.;!?]+)\s*$`)
)

type registerIncomeShortcutGuard struct {
	handle tool.ToolHandle
}

func NewRegisterIncomeShortcutGuard(handle tool.ToolHandle) PreGuard {
	return &registerIncomeShortcutGuard{handle: handle}
}

func (g *registerIncomeShortcutGuard) Name() string {
	return "register_income_shortcut"
}

func (g *registerIncomeShortcutGuard) Inspect(ctx context.Context, in agent.Request) GuardDecision {
	if g.handle == nil {
		return GuardDecision{}
	}
	args, ok := parseRegisterIncomeShortcut(lastUserMessageContent(in.Messages), g.handle)
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
	content := registerIncomeShortcutContent(raw, verbatim)
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

func parseRegisterIncomeShortcut(message string, handle tool.ToolHandle) (map[string]any, bool) {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if !containsAnyText(normalized, "recebi", "ganhei", "caiu", "entrou", "salário", "salario") {
		return nil, false
	}
	amountMatch := incomeAmountRe.FindStringSubmatch(message)
	descriptionMatch := incomeDescriptionRe.FindStringSubmatch(message)
	if len(amountMatch) != 2 || len(descriptionMatch) != 2 {
		return nil, false
	}
	amountCents, ok := parseBrazilianAmountCents(amountMatch[1])
	if !ok {
		return nil, false
	}
	description := strings.TrimSpace(descriptionMatch[1])
	if description == "" {
		return nil, false
	}
	amountArg := any(amountCents)
	if toolPropertyWantsString(handle, "amountCents") {
		amountArg = strconv.FormatInt(amountCents, 10)
	}
	return map[string]any{
		"amountCents": amountArg,
		"description": description,
	}, true
}

func parseBrazilianAmountCents(value string) (int64, bool) {
	clean := strings.ReplaceAll(strings.TrimSpace(value), ".", "")
	clean = strings.ReplaceAll(clean, ",", ".")
	amount, err := strconv.ParseFloat(clean, 64)
	if err != nil || amount <= 0 {
		return 0, false
	}
	return int64(math.Round(amount * 100)), true
}

func registerIncomeShortcutContent(raw []byte, verbatim string) string {
	if strings.TrimSpace(verbatim) != "" {
		return verbatim
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
		return payload.Message
	}
	return "Receita recebida. Vou confirmar esse lançamento antes de concluir. 💰✅"
}
