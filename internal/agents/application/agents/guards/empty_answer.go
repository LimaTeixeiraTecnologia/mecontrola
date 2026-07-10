package guards

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const emptyAnswerFallbackMessage = "Não consegui responder agora. Tente novamente em breve."

type emptyAnswerGuard struct{}

func NewEmptyAnswerGuard() PostGuard {
	return &emptyAnswerGuard{}
}

func (g *emptyAnswerGuard) Name() string {
	return "empty_answer"
}

func (g *emptyAnswerGuard) Inspect(_ context.Context, _ agent.Request, out agent.Result) GuardDecision {
	if strings.TrimSpace(out.Content) != "" {
		return GuardDecision{}
	}
	forced := out
	forced.Content = emptyAnswerFallbackMessage
	return GuardDecision{Handled: true, Result: forced}
}
