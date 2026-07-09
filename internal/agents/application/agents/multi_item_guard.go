package agents

import (
	"context"
	"regexp"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

var (
	multiItemUUIDRe        = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	multiItemDateRe        = regexp.MustCompile(`\d{1,2}/\d{1,2}(?:/\d{2,4})?`)
	multiItemMonthYearRe   = regexp.MustCompile(`(?i)(janeiro|fevereiro|março|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)/\d{2,4}`)
	multiItemIDRe          = regexp.MustCompile(`(?i)[a-z][a-z0-9]*-[a-z0-9-]*\d[a-z0-9-]*`)
	multiItemDayOfRe       = regexp.MustCompile(`(?i)\bdia\s+\d{1,2}\b`)
	multiItemItemSeqRe     = regexp.MustCompile(`(?i)\bitemseq\s+\d+\b`)
	multiItemYearRe        = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	multiItemMoneyWordRe   = regexp.MustCompile(`(?i)\d+\s*(?:reais|real)\b`)
	multiItemPercentRe     = regexp.MustCompile(`(?i)\d+\s*(?:%|por\s*cento|porcento)`)
	multiItemInstallmentRe = regexp.MustCompile(`(?i)\d{1,2}\s*x\b`)
	multiItemOrdinalRe     = regexp.MustCompile(`\d+(?:º|ª)`)
	multiItemMoneyTokenRe  = regexp.MustCompile(`(?i)r\$\s*\d{1,3}(?:\.\d{3})*(?:,\d{1,2})?|\d{1,3}(?:\.\d{3})+,\d{1,2}|\d+,\d{1,2}|\d+\s*(?:reais|real)\b|\d+`)
)

const moneyWordPlaceholder = "\x00MONEYWORD\x00"

func detectMultipleMonetaryValues(message string) bool {
	protectedTokens := multiItemMoneyWordRe.FindAllString(message, -1)
	sanitized := multiItemMoneyWordRe.ReplaceAllString(message, moneyWordPlaceholder)
	sanitized = multiItemUUIDRe.ReplaceAllString(sanitized, " ")
	sanitized = multiItemDateRe.ReplaceAllString(sanitized, " ")
	sanitized = multiItemMonthYearRe.ReplaceAllString(sanitized, " ")
	sanitized = multiItemIDRe.ReplaceAllString(sanitized, " ")
	sanitized = multiItemDayOfRe.ReplaceAllString(sanitized, " ")
	sanitized = multiItemItemSeqRe.ReplaceAllString(sanitized, " ")
	sanitized = multiItemPercentRe.ReplaceAllString(sanitized, " ")
	sanitized = multiItemYearRe.ReplaceAllString(sanitized, " ")
	sanitized = multiItemInstallmentRe.ReplaceAllString(sanitized, " ")
	sanitized = multiItemOrdinalRe.ReplaceAllString(sanitized, " ")
	tokenCount := len(multiItemMoneyTokenRe.FindAllString(sanitized, -1)) + len(protectedTokens)
	return tokenCount >= 2
}

func lastUserMessageContent(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

type multiItemGuardAgent struct {
	agent.Agent
}

func WithMultiItemGuard(a agent.Agent) agent.Agent {
	return &multiItemGuardAgent{Agent: a}
}

func (g *multiItemGuardAgent) Execute(ctx context.Context, in agent.Request) (agent.Result, error) {
	if detectMultipleMonetaryValues(lastUserMessageContent(in.Messages)) {
		return agent.Result{
			Content:     workflows.MultiItemOrientationMessage,
			Mode:        agent.ExecutionModeSync,
			ToolOutcome: agent.ToolOutcomeClarify,
		}, nil
	}
	return g.Agent.Execute(ctx, in)
}
