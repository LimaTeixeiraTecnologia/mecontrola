package guards

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

var (
	budgetWriteTotalHints = []string{
		"orçamento total",
		"orcamento total",
		"valor total do orçamento",
		"valor total do orcamento",
		"aumentar meu orçamento",
		"aumentar meu orcamento",
		"reduzir meu orçamento",
		"reduzir meu orcamento",
	}
	budgetWriteDistributionHints = []string{
		"alterar meus orçamentos para",
		"alterar meus orcamentos para",
		"mudar meus orçamentos para",
		"mudar meus orcamentos para",
		"nova distribuição",
		"nova distribuicao",
		"mudar a distribuição",
		"mudar a distribuicao",
		"alterar a distribuição",
		"alterar a distribuicao",
		"redistribuir orçamento",
		"redistribuir orcamento",
	}
	budgetWriteCategoryHints = []string{
		"custo fixo",
		"custos fixos",
		"conhecimento",
		"prazeres",
		"metas",
		"liberdade financeira",
	}
	budgetWriteMonthExplicitRe = regexp.MustCompile(`(?i)\b(janeiro|fevereiro|mar[cç]o|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)\s+de\s+(20[0-9]{2})\b`)
	budgetWriteMonthNamedRe    = regexp.MustCompile(`(?i)\b(janeiro|fevereiro|mar[cç]o|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)\b`)
	budgetWriteTotalAmountRe   = regexp.MustCompile(`(?i)(?:r\$\s*)?[0-9]{1,3}(?:\.[0-9]{3})*(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?`)
)

type budgetWriteShortcutGuard struct {
	adjustAllocation tool.ToolHandle
	editBudgetTotal  tool.ToolHandle
}

func NewBudgetWriteShortcutGuard(adjustAllocation, editBudgetTotal tool.ToolHandle) PreGuard {
	return &budgetWriteShortcutGuard{
		adjustAllocation: adjustAllocation,
		editBudgetTotal:  editBudgetTotal,
	}
}

func (g *budgetWriteShortcutGuard) Name() string {
	return "budget_write_shortcut"
}

func (g *budgetWriteShortcutGuard) Inspect(ctx context.Context, in agent.Request) GuardDecision {
	message := lastUserMessageContent(in.Messages)
	if args, ok := parseBudgetDistributionShortcut(message); ok && g.adjustAllocation != nil {
		return g.invoke(ctx, g.adjustAllocation, args)
	}
	if args, ok := parseBudgetTotalShortcut(message); ok && g.editBudgetTotal != nil {
		return g.invoke(ctx, g.editBudgetTotal, args)
	}
	return GuardDecision{}
}

func (g *budgetWriteShortcutGuard) invoke(ctx context.Context, handle tool.ToolHandle, args map[string]any) GuardDecision {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return GuardDecision{}
	}
	raw, verbatim, err := handle.Invoke(ctx, argsJSON)
	if err != nil {
		return GuardDecision{InvokeErr: err}
	}
	content := budgetWriteShortcutContent(raw, verbatim)
	return GuardDecision{
		Handled: true,
		Result: agent.Result{
			Content:     content,
			Mode:        agent.ExecutionModeSync,
			ToolOutcome: budgetWriteToolOutcome(raw),
			ToolCalls: []agent.ToolCallRecord{{
				Tool:          handle.ID(),
				Outcome:       agent.ToolCallOutcomeSuccess,
				Content:       string(raw),
				ArgumentsJSON: args,
			}},
		},
	}
}

func parseBudgetDistributionShortcut(message string) (map[string]any, bool) {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return nil, false
	}
	if !containsAnyText(normalized, budgetWriteDistributionHints...) {
		if !containsAnyText(normalized, budgetWriteCategoryHints...) || !strings.Contains(normalized, "%") {
			return nil, false
		}
	}
	return budgetWriteMonthArgs(normalized), true
}

func parseBudgetTotalShortcut(message string) (map[string]any, bool) {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return nil, false
	}
	if !strings.Contains(normalized, "orçamento") && !strings.Contains(normalized, "orcamento") {
		return nil, false
	}
	if containsAnyText(normalized, budgetWriteCategoryHints...) {
		return nil, false
	}
	if !containsAnyText(normalized, budgetWriteTotalHints...) && !budgetWriteTotalAmountRe.MatchString(normalized) {
		return nil, false
	}
	if !containsAnyText(normalized, "mudar", "alterar", "ajustar", "aumentar", "reduzir", "atualizar", "deixar", "definir") {
		return nil, false
	}
	return budgetWriteMonthArgs(normalized), true
}

func budgetWriteMonthArgs(message string) map[string]any {
	args := map[string]any{}
	explicit := budgetWriteMonthExplicitRe.FindStringSubmatch(message)
	if len(explicit) == 3 {
		month, ok := budgetWriteMonthNumber(explicit[1])
		if ok {
			args["monthRefKind"] = "explicit"
			args["month"] = month
			args["year"] = parseBudgetShortcutYear(explicit[2])
			return args
		}
	}
	if containsAnyText(message, "mês passado", "mes passado", "mês anterior", "mes anterior") {
		args["monthRefKind"] = "previous"
		return args
	}
	if containsAnyText(message, "próximo mês", "proximo mes", "mês que vem", "mes que vem") {
		args["monthRefKind"] = "next"
		return args
	}
	named := budgetWriteMonthNamedRe.FindStringSubmatch(message)
	if len(named) == 2 {
		month, ok := budgetWriteMonthNumber(named[1])
		if ok {
			args["monthRefKind"] = "named_without_year"
			args["month"] = month
			return args
		}
	}
	return args
}

func budgetWriteMonthNumber(name string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "janeiro":
		return 1, true
	case "fevereiro":
		return 2, true
	case "março", "marco":
		return 3, true
	case "abril":
		return 4, true
	case "maio":
		return 5, true
	case "junho":
		return 6, true
	case "julho":
		return 7, true
	case "agosto":
		return 8, true
	case "setembro":
		return 9, true
	case "outubro":
		return 10, true
	case "novembro":
		return 11, true
	case "dezembro":
		return 12, true
	default:
		return 0, false
	}
}

func parseBudgetShortcutYear(raw string) int {
	if len(raw) != 4 {
		return 0
	}
	value := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0
		}
		value = value*10 + int(r-'0')
	}
	return value
}

func budgetWriteShortcutContent(raw []byte, verbatim string) string {
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
	return ""
}

func budgetWriteToolOutcome(raw []byte) agent.ToolOutcome {
	var payload struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return agent.ToolOutcomeClarify
	}
	switch strings.TrimSpace(payload.Outcome) {
	case "started", "routed":
		return agent.ToolOutcomeRouted
	default:
		return agent.ToolOutcomeClarify
	}
}
