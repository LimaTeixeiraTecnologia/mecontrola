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
	recurrenceShortcutRe = regexp.MustCompile(`(?i)^\s*todo\s+dia\s+([0-9]{1,2})\s+(?:eu\s+)?(gastei|gasto|pago|paguei|recebo|recebi|ganho|ganhei)\s+(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)\s*(?:reais|real|conto|contos|pila|mango)?\s+(?:no|na|nos|nas|em|com|de|do|da|pra|para)\s+([a-zà-ú][a-zà-ú' ]*?)\s*$`)

	recurrenceShortcutIncomeVerbs = map[string]struct{}{
		"recebo": {},
		"recebi": {},
		"ganho":  {},
		"ganhei": {},
	}
)

type createRecurrenceShortcutGuard struct {
	handle tool.ToolHandle
}

func NewCreateRecurrenceShortcutGuard(handle tool.ToolHandle) PreGuard {
	return &createRecurrenceShortcutGuard{handle: handle}
}

func (g *createRecurrenceShortcutGuard) Name() string {
	return "create_recurrence_shortcut"
}

func (g *createRecurrenceShortcutGuard) Inspect(ctx context.Context, in agent.Request) GuardDecision {
	if g.handle == nil {
		return GuardDecision{}
	}
	args, ok := parseCreateRecurrenceShortcut(lastUserMessageContent(in.Messages), g.handle)
	if !ok {
		return GuardDecision{}
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return GuardDecision{}
	}
	raw, verbatim, err := g.handle.Invoke(ctx, argsJSON)
	if err != nil {
		logShortcutInvokeError(ctx, g.Name(), err)
		return GuardDecision{}
	}
	return GuardDecision{
		Handled: true,
		Result: agent.Result{
			Content:     registerIncomeShortcutContent(raw, verbatim),
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

func parseCreateRecurrenceShortcut(message string, handle tool.ToolHandle) (map[string]any, bool) {
	trimmed := strings.TrimSpace(message)
	normalized := strings.ToLower(trimmed)
	if normalized == "" {
		return nil, false
	}
	body, paymentMethod := splitExpensePaymentSuffix(trimmed)
	normalizedBody := strings.ToLower(body)
	for _, blocker := range expenseShortcutCardContextBlockers {
		if strings.Contains(normalizedBody, blocker) {
			return nil, false
		}
	}
	match := recurrenceShortcutRe.FindStringSubmatch(body)
	if len(match) != 5 {
		return nil, false
	}
	dayOfMonth, err := strconv.Atoi(match[1])
	if err != nil || dayOfMonth < 1 || dayOfMonth > 31 {
		return nil, false
	}
	amountCents, ok := parseBrazilianAmountCents(match[3])
	if !ok {
		return nil, false
	}
	description := strings.TrimSpace(match[4])
	if description == "" {
		return nil, false
	}
	direction := "outcome"
	verb := strings.ToLower(match[2])
	if _, isIncome := recurrenceShortcutIncomeVerbs[verb]; isIncome {
		direction = "income"
	}
	amountArg := any(amountCents)
	if toolPropertyWantsString(handle, "amountCents") {
		amountArg = strconv.FormatInt(amountCents, 10)
	}
	args := map[string]any{
		"direction":   direction,
		"amountCents": amountArg,
		"description": description,
		"frequency":   "monthly",
		"dayOfMonth":  dayOfMonth,
	}
	if paymentMethod != "" {
		args["paymentMethod"] = paymentMethod
	}
	return args, true
}
