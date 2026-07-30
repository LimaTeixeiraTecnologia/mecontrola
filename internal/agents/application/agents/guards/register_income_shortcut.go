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
	incomeAmountRe            = regexp.MustCompile(`(?i)(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)\s*(?:reais|real|reis)?`)
	incomeDescriptionRe       = regexp.MustCompile(`(?i)\bde\s+([^0-9,.;!?]+)\s*$`)
	incomeDescriptionBeforeRe = regexp.MustCompile(`(?i)^\s*(.+?)\s+(?:e\s+)?(?:recebi|ganhei|caiu|entrou)\s+(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)\s*(?:reais|real|reis)?(?:\s+(hoje|ontem))?\s*$`)
	incomeServiceAmountRe     = regexp.MustCompile(`(?i)^\s*(?:novo\s+)?servi[cç]o\s+de\s+(.+?)\s+de\s+(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)\s*(?:reais|real|reis)?(?:\s+(hoje|ontem))?\s*$`)
	incomeSaleRe              = regexp.MustCompile(`(?i)^\s*(?:quero\s+registrar\s+uma\s+)?(?:vendi|venda\s+de|registrar\s+uma\s+venda\s+de)\s+(.+?)\s+(?:por|de)\s+(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)\s*(?:reais|real|reis)?(?:\s+que\s+fiz)?(?:\s+(hoje|ontem))?\s*$`)

	incomeShortcutRecurrenceMarkers = []string{
		"todo dia", "todo mês", "todo mes", "toda semana", "todo ano",
		"mensalmente", "semanalmente", "anualmente", "diariamente",
	}
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
		return GuardDecision{InvokeErr: err}
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
	if !containsAnyText(normalized, "recebi", "ganhei", "caiu", "entrou", "salário", "salario", "vendi", "venda", "serviço", "servico") {
		return nil, false
	}
	if containsAnyText(normalized, incomeShortcutRecurrenceMarkers...) {
		return nil, false
	}
	amountText, description, occurredAt, ok := parseIncomeShortcutParts(message)
	if !ok {
		return nil, false
	}
	amountCents, ok := parseBrazilianAmountCents(amountText)
	if !ok {
		return nil, false
	}
	description = normalizeIncomeDescription(description)
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
	if occurredAt != "" {
		args["occurredAt"] = occurredAt
	}
	return args, true
}

func parseIncomeShortcutParts(message string) (string, string, string, bool) {
	if match := incomeSaleRe.FindStringSubmatch(message); len(match) == 4 {
		return match[2], match[1], strings.TrimSpace(strings.ToLower(match[3])), true
	}
	if match := incomeServiceAmountRe.FindStringSubmatch(message); len(match) == 4 {
		return match[2], match[1], strings.TrimSpace(strings.ToLower(match[3])), true
	}
	if match := incomeDescriptionBeforeRe.FindStringSubmatch(message); len(match) == 4 {
		return match[2], match[1], strings.TrimSpace(strings.ToLower(match[3])), true
	}
	amountMatch := incomeAmountRe.FindStringSubmatch(message)
	descriptionMatch := incomeDescriptionRe.FindStringSubmatch(message)
	if len(amountMatch) != 2 || len(descriptionMatch) != 2 {
		return "", "", "", false
	}
	return amountMatch[1], descriptionMatch[1], "", true
}

func normalizeIncomeDescription(description string) string {
	description = strings.TrimSpace(description)
	prefixes := []string{
		"fiz um serviço de ",
		"fiz uma serviço de ",
		"fiz um servico de ",
		"fiz uma servico de ",
		"fiz uma podologia ",
		"fiz um ",
		"fiz uma ",
		"novo serviço de ",
		"novo servico de ",
		"novo ",
		"serviço de ",
		"servico de ",
	}
	lower := strings.ToLower(description)
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(description[len(prefix):])
		}
	}
	return description
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
