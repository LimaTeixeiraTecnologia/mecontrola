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
	expenseShortcutRe = regexp.MustCompile(`(?i)^\s*(?:hoje\s+|ontem\s+)?(?:gastei|paguei|torrei)\s+(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)\s*(?:reais|real|conto|contos|pila|mango)?\s+(?:no|na|nos|nas|em|com|de|do|da|pra|para)\s+([a-zà-ú][a-zà-ú' ]*?)\s*$`)

	expenseShortcutBlockers = []string{
		"cartão", "cartao", "crédito", "credito", "parcel", "fatura",
		"pix", "débito", "debito", "dinheiro", "espécie", "especie",
		"boleto", "vale", "ted", "doc", "apple pay", "google pay",
		"picpay", "mercado pago", "cheque",
	}
)

type registerExpenseShortcutGuard struct {
	handle tool.ToolHandle
}

func NewRegisterExpenseShortcutGuard(handle tool.ToolHandle) PreGuard {
	return &registerExpenseShortcutGuard{handle: handle}
}

func (g *registerExpenseShortcutGuard) Name() string {
	return "register_expense_shortcut"
}

func (g *registerExpenseShortcutGuard) Inspect(ctx context.Context, in agent.Request) GuardDecision {
	if g.handle == nil {
		return GuardDecision{}
	}
	args, ok := parseRegisterExpenseShortcut(lastUserMessageContent(in.Messages), g.handle)
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

func parseRegisterExpenseShortcut(message string, handle tool.ToolHandle) (map[string]any, bool) {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return nil, false
	}
	for _, blocker := range expenseShortcutBlockers {
		if strings.Contains(normalized, blocker) {
			return nil, false
		}
	}
	match := expenseShortcutRe.FindStringSubmatch(message)
	if len(match) != 3 {
		return nil, false
	}
	amountCents, ok := parseBrazilianAmountCents(match[1])
	if !ok {
		return nil, false
	}
	description := strings.TrimSpace(match[2])
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
