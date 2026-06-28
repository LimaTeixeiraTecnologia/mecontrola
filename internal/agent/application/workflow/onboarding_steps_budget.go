package workflow

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func newBudgetStep(d onboardingDeps) platform.Step[OnboardingState] {
	return platform.NewStepFunc("onboarding.budget", func(ctx context.Context, s OnboardingState) (platform.StepOutput[OnboardingState], error) {
		if s.Inbound == "" {
			return d.suspend(ctx, s, valueobjects.PhaseBudget, AwaitingText, d.Interpreter.RenderBudget(ctx))
		}
		parsed, err := d.Interpreter.ParseBudget(ctx, s.Inbound)
		if err != nil {
			return d.suspend(ctx, s, valueobjects.PhaseBudget, AwaitingText, d.Interpreter.RenderRetry(ctx, "budget"))
		}
		switch DecideBudget(parsed) {
		case OutcomeDeferred:
			d.record(ctx, "budget", "deferred")
			return d.suspend(ctx, s, valueobjects.PhaseBudget, AwaitingText, d.Interpreter.RenderDailyRedirect(ctx, "budget"))
		case OutcomeClarify:
			d.record(ctx, "budget", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseBudget, AwaitingText, d.Interpreter.RenderRetry(ctx, "budget"))
		case OutcomeAdvance:
			if saveErr := d.IncomeSaver.Save(ctx, s.UserID, parsed.IncomeCents); saveErr != nil {
				return fail(saveErr)
			}
			d.record(ctx, "budget", "advance")
			return d.advance(ctx, s, valueobjects.PhaseCards, d.Interpreter.RenderBudgetSaved(ctx, parsed.IncomeCents))
		default:
			d.record(ctx, "budget", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseBudget, AwaitingText, d.Interpreter.RenderRetry(ctx, "budget"))
		}
	})
}
