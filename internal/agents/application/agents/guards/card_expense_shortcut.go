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
	cardExpenseHeadRe = regexp.MustCompile(`(?i)^\s*(?:hoje\s+|ontem\s+)?(?:gastei|paguei|torrei|comprei|parcelei)\s+(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)\s*(?:reais|real|conto|contos|pila|mango)?\s+(.+?)\s*$`)

	cardExpenseInstallmentsRe = regexp.MustCompile(`(?i)\b(?:em\s+)?(\d{1,2})\s*x\b`)

	cardExpenseMarkerRe = regexp.MustCompile(`(?i)\b(?:no|na|em)\s+(?:cart[aã]o(?:\s+de\s+cr[eé]dito)?|cr[eé]dito)(?:\s+(.+?))?\s*$`)

	cardExpenseBlockers = []string{
		"fatura",
	}

	cardExpenseConnectors = []string{
		"no", "na", "nos", "nas", "em", "com", "de", "do", "da", "pra", "para",
	}

	cardExpenseFallbackDescription = "crédito"
)

type cardExpenseParse struct {
	amountCents  int64
	description  string
	nickname     string
	installments int
}

type cardExpenseShortcutGuard struct {
	registerExpense tool.ToolHandle
	resolveCard     tool.ToolHandle
}

func NewCardExpenseShortcutGuard(registerExpense, resolveCard tool.ToolHandle) PreGuard {
	return &cardExpenseShortcutGuard{registerExpense: registerExpense, resolveCard: resolveCard}
}

func (g *cardExpenseShortcutGuard) Name() string {
	return "card_expense_shortcut"
}

func (g *cardExpenseShortcutGuard) Inspect(ctx context.Context, in agent.Request) GuardDecision {
	if g.registerExpense == nil {
		return GuardDecision{}
	}
	parsed, ok := parseCardExpenseShortcut(lastUserMessageContent(in.Messages))
	if !ok {
		return GuardDecision{}
	}
	toolCalls := make([]agent.ToolCallRecord, 0, 2)
	cardID := ""
	if parsed.nickname != "" && g.resolveCard != nil {
		resolveArgs := map[string]any{"nickname": parsed.nickname}
		resolveArgsJSON, err := json.Marshal(resolveArgs)
		if err != nil {
			return GuardDecision{}
		}
		resolveRaw, _, err := g.resolveCard.Invoke(ctx, resolveArgsJSON)
		if err != nil {
			return GuardDecision{InvokeErr: err}
		}
		toolCalls = append(toolCalls, agent.ToolCallRecord{
			Tool:          g.resolveCard.ID(),
			Outcome:       agent.ToolCallOutcomeSuccess,
			Content:       string(resolveRaw),
			ArgumentsJSON: resolveArgs,
		})
		cardID = extractResolvedCardID(resolveRaw)
	}
	args := g.buildRegisterArgs(parsed, cardID)
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return GuardDecision{}
	}
	raw, verbatim, err := g.registerExpense.Invoke(ctx, argsJSON)
	if err != nil {
		return GuardDecision{InvokeErr: err}
	}
	toolCalls = append(toolCalls, agent.ToolCallRecord{
		Tool:          g.registerExpense.ID(),
		Outcome:       agent.ToolCallOutcomeSuccess,
		Content:       string(raw),
		ArgumentsJSON: args,
	})
	return GuardDecision{
		Handled: true,
		Result: agent.Result{
			Content:     registerIncomeShortcutContent(raw, verbatim),
			Mode:        agent.ExecutionModeSync,
			ToolOutcome: agent.ToolOutcomeClarify,
			ToolCalls:   toolCalls,
		},
	}
}

func (g *cardExpenseShortcutGuard) buildRegisterArgs(parsed cardExpenseParse, cardID string) map[string]any {
	amountArg := any(parsed.amountCents)
	if toolPropertyWantsString(g.registerExpense, "amountCents") {
		amountArg = strconv.FormatInt(parsed.amountCents, 10)
	}
	args := map[string]any{
		"amountCents":   amountArg,
		"description":   parsed.description,
		"paymentMethod": "credit_card",
	}
	if cardID != "" {
		args["cardId"] = cardID
	}
	if parsed.installments > 1 {
		args["installments"] = parsed.installments
	}
	return args
}

func parseCardExpenseShortcut(message string) (cardExpenseParse, bool) {
	trimmed := strings.TrimSpace(message)
	normalized := strings.ToLower(trimmed)
	if normalized == "" {
		return cardExpenseParse{}, false
	}
	for _, blocker := range cardExpenseBlockers {
		if strings.Contains(normalized, blocker) {
			return cardExpenseParse{}, false
		}
	}
	head := cardExpenseHeadRe.FindStringSubmatch(trimmed)
	if len(head) != 3 {
		return cardExpenseParse{}, false
	}
	amountCents, ok := parseBrazilianAmountCents(head[1])
	if !ok {
		return cardExpenseParse{}, false
	}
	rest := head[2]
	installments := 0
	if inst := cardExpenseInstallmentsRe.FindStringSubmatchIndex(rest); inst != nil {
		n, err := strconv.Atoi(rest[inst[2]:inst[3]])
		if err != nil || n < 1 || n > 24 {
			return cardExpenseParse{}, false
		}
		installments = n
		rest = strings.TrimSpace(rest[:inst[0]] + " " + rest[inst[1]:])
	}
	marker := cardExpenseMarkerRe.FindStringSubmatchIndex(rest)
	if marker == nil {
		return cardExpenseParse{}, false
	}
	description := trimCardExpenseConnectors(rest[:marker[0]])
	if description == "" {
		description = cardExpenseFallbackDescription
	}
	nickname := ""
	if marker[2] >= 0 {
		nickname = trimCardExpenseConnectors(rest[marker[2]:marker[3]])
		if len(strings.Fields(nickname)) > 3 {
			return cardExpenseParse{}, false
		}
	}
	return cardExpenseParse{
		amountCents:  amountCents,
		description:  description,
		nickname:     nickname,
		installments: installments,
	}, true
}

func trimCardExpenseConnectors(text string) string {
	fields := strings.Fields(text)
	for len(fields) > 0 && isCardExpenseConnector(fields[0]) {
		fields = fields[1:]
	}
	for len(fields) > 0 && isCardExpenseConnector(fields[len(fields)-1]) {
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, " ")
}

func isCardExpenseConnector(word string) bool {
	for _, connector := range cardExpenseConnectors {
		if strings.EqualFold(word, connector) {
			return true
		}
	}
	return false
}

func extractResolvedCardID(raw []byte) string {
	var payload struct {
		Found  bool   `json:"found"`
		CardID string `json:"cardId"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if !payload.Found {
		return ""
	}
	return payload.CardID
}
