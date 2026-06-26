package workflow

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func newCategoriesStep(d onboardingDeps) platform.Step[OnboardingState] {
	return platform.NewStepFunc("onboarding.categories", func(ctx context.Context, s OnboardingState) (platform.StepOutput[OnboardingState], error) {
		if s.Inbound == "" {
			return d.suspend(ctx, s, valueobjects.PhaseCategories, AwaitingText, d.Interpreter.RenderCategories(ctx))
		}
		if isDailyCommand(s.Inbound) {
			d.record(ctx, "categories", "deferred")
			return d.suspend(ctx, s, valueobjects.PhaseCategories, AwaitingText, d.Interpreter.RenderDailyRedirect(ctx, "categories"))
		}
		confirmed, err := d.Interpreter.ParseCategoriesConfirm(ctx, s.Inbound)
		if err != nil {
			d.record(ctx, "categories", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseCategories, AwaitingText, d.Interpreter.RenderRetry(ctx, "categories"))
		}
		if !confirmed {
			d.record(ctx, "categories", "clarify")
			return d.advance(ctx, s, valueobjects.PhaseValues)
		}
		d.record(ctx, "categories", "advance")
		return d.advance(ctx, s, valueobjects.PhaseValues)
	})
}
