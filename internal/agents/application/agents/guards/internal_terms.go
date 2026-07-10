package guards

import (
	"context"
	"regexp"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const internalTermsFallbackMessage = "Não consegui responder agora. Tente novamente em breve."

var internalTermsBlocklist = []string{
	"workflow",
	"pendência",
	"pendencia",
	"correlação",
	"correlacao",
	"correlation",
	"infraestrutura",
	"sistema interno",
	"plataforma",
	"usecase",
	"use case",
	"nil pointer",
	"internal server error",
	"panic",
	"goroutine",
	"stack trace",
	"nullpointer",
	"sql:",
	"context.context",
}

var internalTermsPatterns = compileInternalTermsPatterns(internalTermsBlocklist)

func compileInternalTermsPatterns(terms []string) []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(terms))
	for _, term := range terms {
		patterns = append(patterns, regexp.MustCompile(`\b`+regexp.QuoteMeta(term)+`\b`))
	}
	return patterns
}

type internalTermsGuard struct{}

func NewInternalTermsGuard() PostGuard {
	return &internalTermsGuard{}
}

func (g *internalTermsGuard) Name() string {
	return "internal_terms"
}

func (g *internalTermsGuard) Inspect(_ context.Context, _ agent.Request, out agent.Result) GuardDecision {
	lower := strings.ToLower(out.Content)
	for _, pattern := range internalTermsPatterns {
		if pattern.MatchString(lower) {
			forced := out
			forced.Content = internalTermsFallbackMessage
			return GuardDecision{Handled: true, Result: forced}
		}
	}
	return GuardDecision{}
}
