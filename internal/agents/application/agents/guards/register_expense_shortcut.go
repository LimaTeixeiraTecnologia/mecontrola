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
		"cartão", "cartao", "crédito", "credito", "parcel",
	}

	expenseShortcutCardContextBlockers = []string{
		"cartão", "cartao", "crédito", "credito", "parcel",
	}

	expenseShortcutPayments = []struct {
		phrase string
		method string
	}{
		{"vale refeição", "vale_refeicao"},
		{"vale refeicao", "vale_refeicao"},
		{"vale alimentação", "vale_alimentacao"},
		{"vale alimentacao", "vale_alimentacao"},
		{"vr", "vale_refeicao"},
		{"va", "vale_alimentacao"},
		{"vale", "vale_refeicao"},
		{"débito em conta", "debit_in_account"},
		{"debito em conta", "debit_in_account"},
		{"transferência", "transferencia"},
		{"transferencia", "transferencia"},
		{"mercado pago", "mercado_pago"},
		{"mercadopago", "mercado_pago"},
		{"apple pay", "apple_pay"},
		{"applepay", "apple_pay"},
		{"google pay", "google_pay"},
		{"googlepay", "google_pay"},
		{"dinheiro", "cash"},
		{"espécie", "cash"},
		{"especie", "cash"},
		{"débito", "debit_card"},
		{"debito", "debit_card"},
		{"boleto", "boleto"},
		{"picpay", "picpay"},
		{"cheque", "cheque"},
		{"pix", "pix"},
		{"ted", "ted"},
		{"doc", "doc"},
	}

	expenseShortcutTrailingConnectors = []string{
		"no", "na", "nos", "nas", "em", "com", "de", "do", "da",
		"pra", "para", "pelo", "pela", "via",
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
		return GuardDecision{InvokeErr: err}
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
	trimmed := strings.TrimSpace(message)
	normalized := strings.ToLower(trimmed)
	if normalized == "" {
		return nil, false
	}
	body, paymentMethod := splitExpensePaymentSuffix(trimmed)
	normalizedBody := strings.ToLower(body)
	faturaPayment := strings.Contains(normalizedBody, "fatura")
	for _, blocker := range expenseShortcutBlockers {
		if !strings.Contains(normalizedBody, blocker) {
			continue
		}
		if faturaPayment && isExpenseCardContextBlocker(blocker) {
			continue
		}
		return nil, false
	}
	match := expenseShortcutRe.FindStringSubmatch(body)
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
	args := map[string]any{
		"amountCents": amountArg,
		"description": description,
	}
	if paymentMethod != "" {
		args["paymentMethod"] = paymentMethod
	}
	return args, true
}

func isExpenseCardContextBlocker(blocker string) bool {
	for _, cardBlocker := range expenseShortcutCardContextBlockers {
		if blocker == cardBlocker {
			return true
		}
	}
	return false
}

func splitExpensePaymentSuffix(message string) (string, string) {
	for _, payment := range expenseShortcutPayments {
		body, ok := trimFoldWordSuffix(message, payment.phrase)
		if !ok {
			continue
		}
		for _, connector := range expenseShortcutTrailingConnectors {
			if trimmed, okTrim := trimFoldWordSuffix(body, connector); okTrim {
				body = trimmed
				break
			}
		}
		return body, payment.method
	}
	return message, ""
}

func trimFoldWordSuffix(message string, suffix string) (string, bool) {
	trimmed := strings.TrimRight(message, " ")
	if len(trimmed) < len(suffix) || !strings.EqualFold(trimmed[len(trimmed)-len(suffix):], suffix) {
		return message, false
	}
	idx := len(trimmed) - len(suffix)
	if idx > 0 && trimmed[idx-1] != ' ' {
		return message, false
	}
	return strings.TrimSpace(trimmed[:idx]), true
}
