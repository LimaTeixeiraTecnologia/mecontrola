package guards

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

var (
	queryCardInvoiceIntentRe  = regexp.MustCompile(`(?i)^(?:\s*e\s+)?(?:(?:qual|quanto|como)\b.*\bfatura\b|\be\s+a\s+fatura\b|\bfatura\b.*\?)`)
	queryCardInvoiceCardRefRe = regexp.MustCompile(`(?i)\bcart[aã]o(?:\s+de\s+cr[eé]dito)?\b(.*)$`)
	queryCardInvoiceAmountRe  = regexp.MustCompile(`(?i)(?:r\$\s*)?\d{1,3}(?:\.\d{3})*(?:,\d{1,2})?|(?:r\$\s*)?\d+(?:,\d{1,2})?`)
)

type queryCardInvoiceShortcutGuard struct {
	resolveCard      tool.ToolHandle
	queryCardInvoice tool.ToolHandle
	listCards        tool.ToolHandle
}

type resolvedCardShortcut struct {
	Found  bool   `json:"found"`
	CardID string `json:"cardId"`
}

type queryCardInvoiceShortcutPayload struct {
	RefMonth      string    `json:"refMonth"`
	ItemsTotalBRL string    `json:"itemsTotalBRL"`
	ClosingAt     time.Time `json:"closingAt"`
	DueAt         time.Time `json:"dueAt"`
	Items         []struct {
		ID string `json:"id"`
	} `json:"items"`
	OK bool `json:"ok"`
}

func NewQueryCardInvoiceShortcutGuard(resolveCard, queryCardInvoice, listCards tool.ToolHandle) PreGuard {
	return &queryCardInvoiceShortcutGuard{
		resolveCard:      resolveCard,
		queryCardInvoice: queryCardInvoice,
		listCards:        listCards,
	}
}

func (g *queryCardInvoiceShortcutGuard) Name() string {
	return "query_card_invoice_shortcut"
}

func (g *queryCardInvoiceShortcutGuard) Inspect(ctx context.Context, in agent.Request) GuardDecision {
	if g.resolveCard == nil || g.queryCardInvoice == nil {
		return GuardDecision{}
	}
	nickname, ok := parseQueryCardInvoiceShortcut(lastUserMessageContent(in.Messages))
	if !ok {
		return GuardDecision{}
	}

	resolveArgs := map[string]any{"nickname": nickname}
	resolveArgsJSON, err := json.Marshal(resolveArgs)
	if err != nil {
		return GuardDecision{}
	}
	resolveRaw, _, err := g.resolveCard.Invoke(ctx, resolveArgsJSON)
	if err != nil {
		return GuardDecision{InvokeErr: err}
	}

	toolCalls := []agent.ToolCallRecord{{
		Tool:          g.resolveCard.ID(),
		Outcome:       agent.ToolCallOutcomeSuccess,
		Content:       string(resolveRaw),
		ArgumentsJSON: resolveArgs,
	}}

	resolved, ok := parseResolvedCardShortcut(resolveRaw)
	if !ok {
		return GuardDecision{}
	}
	if !resolved.Found || strings.TrimSpace(resolved.CardID) == "" {
		content := queryCardInvoiceCardNotFoundMessage
		if g.listCards != nil {
			raw, verbatim, invokeErr := g.listCards.Invoke(ctx, []byte("{}"))
			if invokeErr != nil {
				return GuardDecision{InvokeErr: invokeErr}
			}
			toolCalls = append(toolCalls, agent.ToolCallRecord{
				Tool:          g.listCards.ID(),
				Outcome:       agent.ToolCallOutcomeSuccess,
				Content:       string(raw),
				ArgumentsJSON: map[string]any{},
			})
			content = queryCardInvoiceListCardsContent(raw, verbatim)
		}
		return GuardDecision{
			Handled: true,
			Result: agent.Result{
				Content:     content,
				Mode:        agent.ExecutionModeSync,
				ToolOutcome: agent.ToolOutcomeClarify,
				ToolCalls:   toolCalls,
			},
		}
	}

	queryArgs := map[string]any{"cardId": resolved.CardID}
	queryArgsJSON, err := json.Marshal(queryArgs)
	if err != nil {
		return GuardDecision{}
	}
	queryRaw, verbatim, err := g.queryCardInvoice.Invoke(ctx, queryArgsJSON)
	if err != nil {
		return GuardDecision{InvokeErr: err}
	}
	toolCalls = append(toolCalls, agent.ToolCallRecord{
		Tool:          g.queryCardInvoice.ID(),
		Outcome:       agent.ToolCallOutcomeSuccess,
		Content:       string(queryRaw),
		ArgumentsJSON: queryArgs,
	})

	return GuardDecision{
		Handled: true,
		Result: agent.Result{
			Content:     queryCardInvoiceShortcutContent(queryRaw, verbatim),
			Mode:        agent.ExecutionModeSync,
			ToolOutcome: agent.ToolOutcomeRouted,
			ToolCalls:   toolCalls,
		},
	}
}

const queryCardInvoiceCardNotFoundMessage = "❌ Não encontrei esse cartão. Pode me dizer o apelido do cartão (ex.: nubank) para eu localizar o certo?"

func parseQueryCardInvoiceShortcut(message string) (string, bool) {
	trimmed := strings.TrimSpace(message)
	normalized := strings.ToLower(trimmed)
	if normalized == "" {
		return "", false
	}
	if !strings.Contains(normalized, "fatura") {
		return "", false
	}
	if !queryCardInvoiceIntentRe.MatchString(normalized) {
		return "", false
	}
	if strings.Contains(normalized, "gastei") || strings.Contains(normalized, "paguei") || strings.Contains(normalized, "comprei") || strings.Contains(normalized, "parcelei") || strings.Contains(normalized, "recebi") {
		return "", false
	}
	if queryCardInvoiceAmountRe.MatchString(normalized) {
		return "", false
	}
	match := queryCardInvoiceCardRefRe.FindStringSubmatch(trimmed)
	if len(match) != 2 {
		return "", false
	}
	nickname := trimInvoiceNickname(match[1])
	if nickname == "" {
		return "", false
	}
	if len(strings.Fields(nickname)) > 3 {
		return "", false
	}
	return nickname, true
}

func trimInvoiceNickname(raw string) string {
	cleaned := strings.TrimSpace(strings.TrimRight(raw, "?.!,:;"))
	fields := strings.Fields(cleaned)
	for len(fields) > 0 {
		lowered := strings.ToLower(fields[0])
		if lowered != "do" && lowered != "da" && lowered != "de" && lowered != "meu" && lowered != "minha" && lowered != "o" && lowered != "a" {
			break
		}
		fields = fields[1:]
	}
	return strings.TrimSpace(strings.Join(fields, " "))
}

func parseResolvedCardShortcut(raw []byte) (resolvedCardShortcut, bool) {
	var payload resolvedCardShortcut
	if err := json.Unmarshal(raw, &payload); err != nil {
		return resolvedCardShortcut{}, false
	}
	return payload, true
}

func queryCardInvoiceShortcutContent(raw []byte, verbatim string) string {
	if strings.TrimSpace(verbatim) != "" {
		return verbatim
	}
	var payload queryCardInvoiceShortcutPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "Consultei a fatura do cartão solicitado. 💳"
	}
	if payload.RefMonth == "" || strings.TrimSpace(payload.ItemsTotalBRL) == "" {
		if payload.OK {
			return "Consultei a fatura do cartão solicitado. 💳"
		}
		return "Não consegui consultar a fatura agora. Tente novamente em breve."
	}
	lines := []string{
		fmt.Sprintf("💳 A fatura de *%s* está em *%s*.", describeInvoiceRefMonth(payload.RefMonth), payload.ItemsTotalBRL),
	}
	if !payload.ClosingAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Fechamento: *%s*", payload.ClosingAt.In(loadBrazilLocation()).Format("02/01")))
	}
	if !payload.DueAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Vencimento: *%s*", payload.DueAt.In(loadBrazilLocation()).Format("02/01")))
	}
	lines = append(lines, fmt.Sprintf("Itens lançados: *%d*.", len(payload.Items)))
	return strings.Join(lines, "\n")
}

func queryCardInvoiceListCardsContent(raw []byte, verbatim string) string {
	content := listCardsShortcutContent(raw, verbatim)
	if strings.TrimSpace(content) == "" {
		return queryCardInvoiceCardNotFoundMessage
	}
	if strings.HasPrefix(content, "Aqui estão seus") || strings.HasPrefix(content, "Você ainda não tem") {
		return "❌ Não encontrei esse cartão.\n\n" + content
	}
	return content
}

func describeInvoiceRefMonth(refMonth string) string {
	t, err := time.Parse("2006-01", refMonth)
	if err != nil {
		return refMonth
	}
	months := [...]string{
		"janeiro", "fevereiro", "março", "abril", "maio", "junho",
		"julho", "agosto", "setembro", "outubro", "novembro", "dezembro",
	}
	return fmt.Sprintf("%s de %d", months[t.Month()-1], t.Year())
}

func loadBrazilLocation() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.UTC
	}
	return loc
}
