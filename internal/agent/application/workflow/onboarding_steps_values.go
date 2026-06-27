package workflow

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func newValuesStep(d onboardingDeps) platform.Step[OnboardingState] {
	return platform.NewStepFunc("onboarding.values", func(ctx context.Context, s OnboardingState) (platform.StepOutput[OnboardingState], error) {
		if s.Values == nil {
			s.Values = make(map[string]int64)
		}
		pending := pendingCategory(s.Values)
		if s.Inbound == "" {
			return d.suspend(ctx, s, valueobjects.PhaseValues, AwaitingText, d.Interpreter.RenderValues(ctx, pending))
		}
		parsed, err := d.Interpreter.ParseValue(ctx, s.Inbound)
		if err != nil {
			return fail(err)
		}
		if parsed.DailyCommand {
			d.record(ctx, "values", "deferred")
			return d.suspend(ctx, s, valueobjects.PhaseValues, AwaitingText, d.Interpreter.RenderDailyRedirect(ctx, "values"))
		}
		if parsed.Ambiguous {
			d.record(ctx, "values", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseValues, AwaitingText, d.Interpreter.RenderRetry(ctx, "values"))
		}
		candidate := copyValues(s.Values)
		candidate[pending] = parsed.ValueCents
		if next := pendingCategory(candidate); next != "" {
			s.Values = candidate
			s.Ack = d.Interpreter.RenderValueSaved(ctx, pending, parsed.ValueCents)
			return d.suspend(ctx, s, valueobjects.PhaseValues, AwaitingText, d.Interpreter.RenderValues(ctx, next))
		}
		contextState, err := d.ContextLoader.Load(ctx, s.UserID)
		if err != nil {
			return fail(err)
		}
		state := BuildValuesState(candidate, contextState.IncomeCents)
		if DecideValues(state) != OutcomeAdvance {
			d.record(ctx, "values", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseValues, AwaitingText, d.Interpreter.RenderValuesMismatch(ctx, sumValues(candidate), contextState.IncomeCents))
		}
		applied, err := d.SplitsSaver.Save(ctx, s.UserID, candidate)
		if err != nil {
			return fail(err)
		}
		if !applied {
			d.record(ctx, "values", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseValues, AwaitingText, d.Interpreter.RenderRetry(ctx, "values"))
		}
		s.Values = candidate
		d.record(ctx, "values", "advance")
		return d.advance(ctx, s, valueobjects.PhaseSummary, d.Interpreter.RenderValueSaved(ctx, pending, parsed.ValueCents))
	})
}
