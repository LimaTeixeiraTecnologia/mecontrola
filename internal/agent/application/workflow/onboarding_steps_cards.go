package workflow

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func newCardsStep(d onboardingDeps) platform.Step[OnboardingState] {
	return platform.NewStepFunc("onboarding.cards", func(ctx context.Context, s OnboardingState) (platform.StepOutput[OnboardingState], error) {
		if s.Inbound == "" {
			return d.suspend(ctx, s, valueobjects.PhaseCards, AwaitingText, d.Interpreter.RenderCards(ctx, s.CardLoop))
		}
		parsed, err := d.Interpreter.ParseCards(ctx, s.Inbound, s.CardLoop)
		if err != nil {
			return d.suspend(ctx, s, valueobjects.PhaseCards, AwaitingText, d.Interpreter.RenderRetry(ctx, "cards"))
		}
		switch DecideCards(parsed) {
		case OutcomeDeferred:
			d.record(ctx, "cards", "deferred")
			return d.suspend(ctx, s, valueobjects.PhaseCards, AwaitingText, d.Interpreter.RenderDailyRedirect(ctx, "cards"))
		case OutcomeClarify:
			d.record(ctx, "cards", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseCards, AwaitingText, d.Interpreter.RenderRetry(ctx, "cards"))
		case OutcomeAdvance:
			if parsed.Skip {
				d.record(ctx, "cards", "advance")
				return d.advance(ctx, s, valueobjects.PhaseCategories, "")
			}
			if parsed.AddAnother {
				d.record(ctx, "cards", "advance")
				return d.suspend(ctx, s, valueobjects.PhaseCards, AwaitingText, d.Interpreter.RenderCards(ctx, s.CardLoop))
			}
			if parsed.Nickname != "" && parsed.DueDay >= 1 && parsed.DueDay <= 31 {
				if saveErr := d.CardSaver.Save(ctx, s.UserID, parsed.Nickname, parsed.DueDay); saveErr != nil {
					return fail(saveErr)
				}
				s.CardLoop++
				s.Ack = d.Interpreter.RenderCardSaved(ctx, parsed.Nickname, parsed.DueDay)
				d.record(ctx, "cards", "advance")
				return d.suspend(ctx, s, valueobjects.PhaseCards, AwaitingText, d.Interpreter.RenderCards(ctx, s.CardLoop))
			}
			d.record(ctx, "cards", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseCards, AwaitingText, d.Interpreter.RenderRetry(ctx, "cards"))
		default:
			d.record(ctx, "cards", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseCards, AwaitingText, d.Interpreter.RenderRetry(ctx, "cards"))
		}
	})
}
