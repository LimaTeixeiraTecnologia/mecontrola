package workflow

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func newWelcomeStep(d onboardingDeps) platform.Step[OnboardingState] {
	return platform.NewStepFunc("onboarding.welcome", func(ctx context.Context, s OnboardingState) (platform.StepOutput[OnboardingState], error) {
		if s.Inbound == "" || s.Inbound == OnboardingWelcomeSignal {
			alreadySent, err := d.WelcomeMarker.Mark(ctx, s.UserID)
			if err != nil {
				return fail(err)
			}
			if alreadySent {
				d.record(ctx, "welcome", "advance")
				return d.advance(ctx, s, valueobjects.PhaseObjective)
			}
			return d.suspend(ctx, s, valueobjects.PhaseWelcome, AwaitingText, d.Interpreter.RenderWelcome(ctx))
		}
		text := strings.ToLower(strings.TrimSpace(s.Inbound))
		if isDailyCommand(text) {
			d.record(ctx, "welcome", "deferred")
			return d.suspend(ctx, s, valueobjects.PhaseWelcome, AwaitingText, d.Interpreter.RenderDailyRedirect(ctx, "welcome"))
		}
		if isConfirmation(text) {
			d.record(ctx, "welcome", "advance")
			return d.advance(ctx, s, valueobjects.PhaseObjective)
		}
		d.record(ctx, "welcome", "clarify")
		return d.suspend(ctx, s, valueobjects.PhaseWelcome, AwaitingText, d.Interpreter.RenderRetry(ctx, "welcome"))
	})
}

func isConfirmation(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	switch text {
	case "sim", "yes", "vamos", "começar", "bora", "ok", "certo":
		return true
	default:
		return false
	}
}

func isDailyCommand(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	cues := []string{
		"gastei", "gasto", "comprei", "paguei", "recebi",
		"quanto", "qual meu", "meu saldo", "como estou", "resumo do mês",
	}
	for _, cue := range cues {
		if strings.Contains(text, cue) {
			return true
		}
	}
	return false
}
