package workflow

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func newObjectiveStep(d onboardingDeps) platform.Step[OnboardingState] {
	return platform.NewStepFunc("onboarding.objective", func(ctx context.Context, s OnboardingState) (platform.StepOutput[OnboardingState], error) {
		if s.Inbound == "" {
			return d.suspend(ctx, s, valueobjects.PhaseObjective, AwaitingText, d.Interpreter.RenderObjective(ctx))
		}
		parsed, err := d.Interpreter.ParseObjective(ctx, s.Inbound)
		if err != nil {
			return d.suspend(ctx, s, valueobjects.PhaseObjective, AwaitingText, d.Interpreter.RenderRetry(ctx, "objective"))
		}
		switch DecideObjective(parsed) {
		case OutcomeDeferred:
			d.record(ctx, "objective", "deferred")
			return d.suspend(ctx, s, valueobjects.PhaseObjective, AwaitingText, d.Interpreter.RenderDailyRedirect(ctx, "objective"))
		case OutcomeClarify:
			d.record(ctx, "objective", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseObjective, AwaitingText, d.Interpreter.RenderRetry(ctx, "objective"))
		case OutcomeAdvance:
			if saveErr := d.ObjectiveSaver.Save(ctx, s.UserID, parsed.Objective); saveErr != nil {
				return fail(saveErr)
			}
			d.record(ctx, "objective", "advance")
			return d.advance(ctx, s, valueobjects.PhaseBudget)
		default:
			d.record(ctx, "objective", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseObjective, AwaitingText, d.Interpreter.RenderRetry(ctx, "objective"))
		}
	})
}
