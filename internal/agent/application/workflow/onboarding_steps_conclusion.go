package workflow

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func newConclusionStep(d onboardingDeps) platform.Step[OnboardingState] {
	return platform.NewStepFunc("onboarding.conclusion", func(ctx context.Context, s OnboardingState) (platform.StepOutput[OnboardingState], error) {
		if err := d.SessionCompleter.Complete(ctx, s.UserID); err != nil {
			return fail(err)
		}
		if err := d.Set(ctx, s.UserID, valueobjects.PhaseConclusion.String()); err != nil {
			return fail(err)
		}
		d.record(ctx, "conclusion", "advance")
		return platform.StepOutput[OnboardingState]{
			State:   s,
			Status:  platform.StepStatusCompleted,
			Suspend: &platform.Suspension{Reason: platform.SuspendAwaitingInput, Prompt: d.Interpreter.RenderConclusion(ctx)},
		}, nil
	})
}
