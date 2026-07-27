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
	editEntryTwoAmountsLancamentoRe = regexp.MustCompile(`(?i)lan[çc]amento\s+(?:d[ao]|de)\s+([a-zà-ú][a-zà-ú\s]{1,30}?)\s*,?\s+.*?valor\s*(?:certo)?\s*(?:é|eh)?\s*(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})+(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)\s+e\s+n[ãa]o\s+(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})+(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)`)
	editEntryTwoAmountsGasteiRe     = regexp.MustCompile(`(?i)\bn[ao]\s+([a-zà-ú][a-zà-ú\s]{1,30}?)\s+(?:eu\s+)?gastei\s+(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})+(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)\s+e\s+n[ãa]o\s+(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})+(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)`)
	editEntryNewValueOnlyRe         = regexp.MustCompile(`(?i)lan[çc]amento\s+(?:d[ao]|de)\s+([a-zà-ú][a-zà-ú\s]{1,30}?)\s*,?\s+.*?(?:mud[ae]r?|altera[re]?|atualiza[re]?)\s+(?:o\s+)?valor\s+(?:para|pra)\s+(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})+(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)`)

	editEntryShortcutBlockers = []string{
		"cartão", "cartao", "orçamento", "orcamento", "objetivo",
		"tratamento", "chamar", "senha", "vencimento", "apelido",
		"recorrência", "recorrencia",
	}
)

type editEntryCorrectionShortcutGuard struct {
	handle tool.ToolHandle
}

func NewEditEntryCorrectionShortcutGuard(handle tool.ToolHandle) PreGuard {
	return &editEntryCorrectionShortcutGuard{handle: handle}
}

func (g *editEntryCorrectionShortcutGuard) Name() string {
	return "edit_entry_correction_shortcut"
}

func (g *editEntryCorrectionShortcutGuard) Inspect(ctx context.Context, in agent.Request) GuardDecision {
	if g.handle == nil {
		return GuardDecision{}
	}
	args, ok := parseEditEntryCorrectionShortcut(lastUserMessageContent(in.Messages), g.handle)
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
			Content:     editEntryShortcutContent(raw, verbatim),
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

func parseEditEntryCorrectionShortcut(message string, handle tool.ToolHandle) (map[string]any, bool) {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return nil, false
	}
	for _, blocker := range editEntryShortcutBlockers {
		if strings.Contains(normalized, blocker) {
			return nil, false
		}
	}

	if match := editEntryTwoAmountsLancamentoRe.FindStringSubmatch(message); len(match) == 4 {
		return buildEditEntryCorrectionArgs(match[1], match[2], match[3], handle)
	}
	if match := editEntryTwoAmountsGasteiRe.FindStringSubmatch(message); len(match) == 4 {
		return buildEditEntryCorrectionArgs(match[1], match[2], match[3], handle)
	}
	if match := editEntryNewValueOnlyRe.FindStringSubmatch(message); len(match) == 3 {
		return buildEditEntryCorrectionArgs(match[1], match[2], "", handle)
	}
	return nil, false
}

func buildEditEntryCorrectionArgs(desc, newValue, oldValue string, handle tool.ToolHandle) (map[string]any, bool) {
	description := strings.TrimSpace(desc)
	if description == "" {
		return nil, false
	}
	newCents, ok := parseBrazilianAmountCents(newValue)
	if !ok {
		return nil, false
	}

	args := map[string]any{
		"searchTerm":  description,
		"amountCents": toolAmountArg(handle, "amountCents", newCents),
	}
	if oldValue != "" {
		if oldCents, oldOk := parseBrazilianAmountCents(oldValue); oldOk {
			args["searchAmountCents"] = toolAmountArg(handle, "searchAmountCents", oldCents)
		}
	}
	return args, true
}

func toolAmountArg(handle tool.ToolHandle, property string, cents int64) any {
	if toolPropertyWantsString(handle, property) {
		return strconv.FormatInt(cents, 10)
	}
	return cents
}

func editEntryShortcutContent(raw []byte, verbatim string) string {
	if strings.TrimSpace(verbatim) != "" {
		return verbatim
	}
	var payload struct {
		ImpactNote string `json:"impactNote"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil && strings.TrimSpace(payload.ImpactNote) != "" {
		return payload.ImpactNote
	}
	return "Este lançamento será atualizado com os novos dados. Por favor confirme."
}
