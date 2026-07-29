package guards

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type dayQueryKind string

const (
	dayQueryExpense dayQueryKind = "expense"
	dayQueryIncome  dayQueryKind = "income"
)

var (
	queryDayAmountRe = regexp.MustCompile(`(?i)^\s*(?:por\s+favor[,]?\s*)?(?:me\s+(?:diz|fala|conta)[,]?\s*)?quant[oa]s?\s+(?:eu\s+)?(gastei|recebi)(?:\s+no\s+total)?\s+(hoje|ontem)[\s?.!]*$`)
	queryDayWhatRe   = regexp.MustCompile(`(?i)^\s*(?:o\s+que|oq)\s+(?:eu\s+)?(gastei|recebi)\s+(hoje|ontem)[\s?.!]*$`)
	queryDayListRe   = regexp.MustCompile(`(?i)^\s*(?:quais|qual)\s+(?:foram\s+)?(?:(?:os|as|meus|minhas)\s+)?(gastos|recebimentos)\s+(?:de\s+)?(hoje|ontem)[\s?.!]*$`)
)

type queryDayShortcutGuard struct {
	handle tool.ToolHandle
}

func NewQueryDayShortcutGuard(handle tool.ToolHandle) PreGuard {
	return &queryDayShortcutGuard{handle: handle}
}

func (g *queryDayShortcutGuard) Name() string {
	return "query_day_shortcut"
}

func (g *queryDayShortcutGuard) Inspect(ctx context.Context, in agent.Request) GuardDecision {
	if g.handle == nil {
		return GuardDecision{}
	}
	kind, dayRef, ok := parseQueryDayShortcut(lastUserMessageContent(in.Messages))
	if !ok {
		return GuardDecision{}
	}
	dayRefKind := "today"
	if dayRef == "ontem" {
		dayRefKind = "yesterday"
	}
	args := map[string]any{"dayRefKind": dayRefKind}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return GuardDecision{}
	}
	raw, _, err := g.handle.Invoke(ctx, argsJSON)
	if err != nil {
		return GuardDecision{}
	}
	content, ok := buildQueryDayContent(raw, kind, dayRef)
	if !ok {
		return GuardDecision{}
	}
	return GuardDecision{
		Handled: true,
		Result: agent.Result{
			Content:     content,
			Mode:        agent.ExecutionModeSync,
			ToolOutcome: agent.ToolOutcomeRouted,
			ToolCalls: []agent.ToolCallRecord{{
				Tool:          g.handle.ID(),
				Outcome:       agent.ToolCallOutcomeSuccess,
				Content:       string(raw),
				ArgumentsJSON: args,
			}},
		},
	}
}

func parseQueryDayShortcut(message string) (dayQueryKind, string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return "", "", false
	}
	if m := queryDayAmountRe.FindStringSubmatch(normalized); len(m) == 3 {
		return dayQueryKindFor(m[1]), m[2], true
	}
	if m := queryDayWhatRe.FindStringSubmatch(normalized); len(m) == 3 {
		return dayQueryKindFor(m[1]), m[2], true
	}
	if m := queryDayListRe.FindStringSubmatch(normalized); len(m) == 3 {
		return dayQueryKindFor(m[1]), m[2], true
	}
	return "", "", false
}

func dayQueryKindFor(verb string) dayQueryKind {
	if verb == "recebi" || verb == "recebimentos" {
		return dayQueryIncome
	}
	return dayQueryExpense
}

type queryDayShortcutEntry struct {
	AmountBRL   string `json:"amountBRL"`
	Direction   string `json:"direction"`
	Description string `json:"description"`
}

type queryDayShortcutPayload struct {
	OutcomeBRL string                  `json:"outcomeBRL"`
	IncomeBRL  string                  `json:"incomeBRL"`
	Entries    []queryDayShortcutEntry `json:"entries"`
}

func buildQueryDayContent(raw []byte, kind dayQueryKind, dayRef string) (string, bool) {
	var payload queryDayShortcutPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", false
	}
	dayLabel := "Hoje"
	if dayRef == "ontem" {
		dayLabel = "Ontem"
	}
	total := payload.OutcomeBRL
	verb := "gastou"
	noun := "gastos"
	empty := "Não há gastos até o momento."
	emoji := "💰"
	direction := "outcome"
	if kind == dayQueryIncome {
		total = payload.IncomeBRL
		verb = "recebeu"
		noun = "recebimentos"
		empty = "Não há recebimentos até o momento."
		emoji = "🎉"
		direction = "income"
	}
	if strings.TrimSpace(total) == "" {
		return "", false
	}
	var lines []string
	for _, e := range payload.Entries {
		if e.Direction != direction || strings.TrimSpace(e.AmountBRL) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%d. *%s* — %s", len(lines)+1, e.AmountBRL, e.Description))
	}
	var b strings.Builder
	if len(lines) == 0 {
		fmt.Fprintf(&b, "%s, você não teve lançamentos registrados. %s\n\nSe precisar de mais alguma coisa, é só avisar! 😊", dayLabel, empty)
		return b.String(), true
	}
	fmt.Fprintf(&b, "%s, você %s um total de *%s*. %s\n\nAqui estão os detalhes dos seus %s:\n\n%s\n\nSe precisar de mais alguma coisa, é só avisar! 😊", dayLabel, verb, total, emoji, noun, strings.Join(lines, "\n"))
	return b.String(), true
}
