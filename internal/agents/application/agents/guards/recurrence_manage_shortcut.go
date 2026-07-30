package guards

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

var (
	recurrenceManageCancelRe = regexp.MustCompile(`(?i)(?:cancel[ae]r?|encerr[ae]r?|remov[ae]r?|exclu[ai]r?|apag[au]r?)\s+(?:a\s+)?recorr[êe]ncia\s+(?:d[oa]\s+)?(.+?)\s*$`)
	recurrenceManageUpdateRe = regexp.MustCompile(`(?i)(?:mud[ae]r?|alter[ae]r?|atualiz[ae]r?)\s+(?:a\s+recorr[êe]ncia\s+)?(?:d[oa]\s+|o\s+|a\s+)?(.+?)\s+para\s+(?:r\$\s*)?([0-9]{1,3}(?:\.[0-9]{3})+(?:,[0-9]{1,2})?|[0-9]+(?:,[0-9]{1,2})?)\s*$`)
	recurrenceManageDayRe    = regexp.MustCompile(`(?i)\b(?:do\s+)?dia\s+(\d{1,2})\b`)

	recurrenceManageBlockers = []string{
		"lançamento", "lancamento", "orçamento", "orcamento",
		"fatura", "cartão", "cartao", "senha", "tratamento",
	}
)

type recurrenceManageShortcutGuard struct {
	updateHandle tool.ToolHandle
	deleteHandle tool.ToolHandle
	listHandle   tool.ToolHandle
}

func NewRecurrenceManageShortcutGuard(updateHandle, deleteHandle, listHandle tool.ToolHandle) PreGuard {
	return &recurrenceManageShortcutGuard{updateHandle: updateHandle, deleteHandle: deleteHandle, listHandle: listHandle}
}

func (g *recurrenceManageShortcutGuard) Name() string {
	return "recurrence_manage_shortcut"
}

func (g *recurrenceManageShortcutGuard) Inspect(ctx context.Context, in agent.Request) GuardDecision {
	if g.listHandle == nil || (g.updateHandle == nil && g.deleteHandle == nil) {
		return GuardDecision{}
	}
	message := lastUserMessageContent(in.Messages)
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" || recurrenceManageBlocked(normalized) {
		return GuardDecision{}
	}

	if g.deleteHandle != nil {
		if match := recurrenceManageCancelRe.FindStringSubmatch(message); len(match) == 2 {
			return g.invokeResolved(ctx, match[1], g.deleteHandle, nil)
		}
	}
	if g.updateHandle != nil {
		if match := recurrenceManageUpdateRe.FindStringSubmatch(message); len(match) == 3 {
			cents, ok := parseBrazilianAmountCents(match[2])
			if ok {
				return g.invokeResolved(ctx, match[1], g.updateHandle, map[string]any{"amountCents": cents})
			}
		}
	}
	return GuardDecision{}
}

func recurrenceManageBlocked(normalized string) bool {
	if !strings.Contains(normalized, "recorrência") && !strings.Contains(normalized, "recorrencia") {
		for _, blocker := range recurrenceManageBlockers {
			if strings.Contains(normalized, blocker) {
				return true
			}
		}
	}
	return false
}

type recurrenceManageCandidate struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	DayOfMonth  int    `json:"dayOfMonth"`
	Version     int64  `json:"version"`
}

func (g *recurrenceManageShortcutGuard) invokeResolved(ctx context.Context, rawTerm string, handle tool.ToolHandle, extra map[string]any) GuardDecision {
	candidates, err := g.listCandidates(ctx)
	if err != nil {
		return GuardDecision{InvokeErr: err}
	}
	candidate, ok := resolveRecurrenceManageCandidate(candidates, rawTerm)
	if !ok {
		return GuardDecision{}
	}

	args := map[string]any{
		"templateId": candidate.ID,
		"version":    candidate.Version,
	}
	for k, v := range extra {
		args[k] = v
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return GuardDecision{}
	}
	raw, verbatim, err := handle.Invoke(ctx, argsJSON)
	if err != nil {
		return GuardDecision{InvokeErr: err}
	}
	return GuardDecision{
		Handled: true,
		Result: agent.Result{
			Content:     recurrenceManageContent(raw, verbatim),
			Mode:        agent.ExecutionModeSync,
			ToolOutcome: agent.ToolOutcomeClarify,
			ToolCalls: []agent.ToolCallRecord{{
				Tool:          handle.ID(),
				Outcome:       agent.ToolCallOutcomeSuccess,
				Content:       string(raw),
				ArgumentsJSON: args,
			}},
		},
	}
}

func (g *recurrenceManageShortcutGuard) listCandidates(ctx context.Context) ([]recurrenceManageCandidate, error) {
	raw, _, err := g.listHandle.Invoke(ctx, []byte(`{"activeOnly":true}`))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Recurrences []recurrenceManageCandidate `json:"recurrences"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload.Recurrences, nil
}

func resolveRecurrenceManageCandidate(candidates []recurrenceManageCandidate, rawTerm string) (recurrenceManageCandidate, bool) {
	term := strings.TrimSpace(rawTerm)
	day := 0
	if match := recurrenceManageDayRe.FindStringSubmatch(term); len(match) == 2 {
		if parsed, err := strconv.Atoi(match[1]); err == nil {
			day = parsed
			term = strings.TrimSpace(recurrenceManageDayRe.ReplaceAllString(term, ""))
		}
	}
	termNorm := normalizeRecurrenceManageTerm(term)
	if termNorm == "" {
		return recurrenceManageCandidate{}, false
	}

	var matches []recurrenceManageCandidate
	for _, c := range candidates {
		if day != 0 && c.DayOfMonth != day {
			continue
		}
		descNorm := normalizeRecurrenceManageTerm(c.Description)
		if descNorm == termNorm || strings.Contains(descNorm, termNorm) || strings.Contains(termNorm, descNorm) {
			matches = append(matches, c)
		}
	}
	if len(matches) != 1 {
		return recurrenceManageCandidate{}, false
	}
	return matches[0], true
}

func normalizeRecurrenceManageTerm(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	decomposed := norm.NFD.String(s)
	var b strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func recurrenceManageContent(raw []byte, verbatim string) string {
	if strings.TrimSpace(verbatim) != "" {
		return verbatim
	}
	var payload struct {
		ImpactNote string `json:"impactNote"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil && strings.TrimSpace(payload.ImpactNote) != "" {
		return payload.ImpactNote
	}
	return "Esta recorrência será atualizada com os novos dados. Por favor confirme."
}
