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

var (
	listCardsIntentRe = regexp.MustCompile(`(?i)(quais|qual|liste|lista|listar|mostrar|mostre|ver|meus|seus|tenho|possui)\s+(são\s+)?(meus\s+|os\s+|seus\s+)?(💳|cart[ãa]o|cart[õo]es)`)
)

type listCardsShortcutGuard struct {
	handle tool.ToolHandle
}

func NewListCardsShortcutGuard(handle tool.ToolHandle) PreGuard {
	return &listCardsShortcutGuard{handle: handle}
}

func (g *listCardsShortcutGuard) Name() string {
	return "list_cards_shortcut"
}

func (g *listCardsShortcutGuard) Inspect(ctx context.Context, in agent.Request) GuardDecision {
	if g.handle == nil {
		return GuardDecision{}
	}
	if !isListCardsRequest(lastUserMessageContent(in.Messages)) {
		return GuardDecision{}
	}
	raw, verbatim, err := g.handle.Invoke(ctx, []byte("{}"))
	if err != nil {
		return GuardDecision{}
	}
	content := listCardsShortcutContent(raw, verbatim)
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
				ArgumentsJSON: map[string]any{},
			}},
		},
	}
}

func isListCardsRequest(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if !containsAnyText(normalized, "💳", "cartao", "cartão", "cartoes", "cartões") {
		return false
	}
	if containsAnyText(normalized, "fatura", "compr", "gast", "pag", "cadastr", "criar", "adicionar", "adiciona", "vencimento", "melhor dia", "parcel") {
		return false
	}
	return listCardsIntentRe.MatchString(normalized)
}

func listCardsShortcutContent(raw []byte, verbatim string) string {
	if strings.TrimSpace(verbatim) != "" {
		return verbatim
	}
	var payload struct {
		Cards []struct {
			Nickname string `json:"nickname"`
			Bank     string `json:"bank"`
			DueDay   int    `json:"dueDay"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "Não consegui consultar seus 💳 agora. Tente novamente em breve."
	}
	if len(payload.Cards) == 0 {
		return "Você ainda não tem nenhum 💳 cadastrado."
	}
	var b strings.Builder
	b.WriteString("Aqui estão seus 💳:\n\n")
	for i, card := range payload.Cards {
		if i > 0 {
			b.WriteString("\n")
		}
		if card.Nickname == card.Bank {
			fmt.Fprintf(&b, "• %s — vencimento dia %d", card.Bank, card.DueDay)
			continue
		}
		fmt.Fprintf(&b, "• %s (%s) — vencimento dia %d", card.Nickname, card.Bank, card.DueDay)
	}
	return b.String()
}
