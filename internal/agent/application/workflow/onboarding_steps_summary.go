package workflow

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func newSummaryStep(d onboardingDeps) platform.Step[OnboardingState] {
	return platform.NewStepFunc("onboarding.summary", func(ctx context.Context, s OnboardingState) (platform.StepOutput[OnboardingState], error) {
		contextState, err := d.ContextLoader.Load(ctx, s.UserID)
		if err != nil {
			return fail(err)
		}
		if s.Inbound == "" {
			return d.suspend(ctx, s, valueobjects.PhaseSummary, AwaitingConfirm, d.Interpreter.RenderSummary(ctx, summaryState(s, contextState)))
		}
		parsed, err := d.Interpreter.ParseSummary(ctx, s.Inbound)
		if err != nil {
			d.record(ctx, "summary", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseSummary, AwaitingConfirm, d.Interpreter.RenderRetry(ctx, "summary"))
		}
		switch DecideSummary(parsed) {
		case OutcomeDeferred:
			d.record(ctx, "summary", "deferred")
			return d.suspend(ctx, s, valueobjects.PhaseSummary, AwaitingConfirm, d.Interpreter.RenderDailyRedirect(ctx, "summary"))
		case OutcomeClarify:
			d.record(ctx, "summary", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseSummary, AwaitingConfirm, d.Interpreter.RenderRetry(ctx, "summary"))
		case OutcomeCancel:
			d.record(ctx, "summary", "clarify")
			return d.suspend(ctx, s, valueobjects.PhaseSummary, AwaitingConfirm, d.Interpreter.RenderRetry(ctx, "summary"))
		case OutcomeCorrect:
			return applyCorrection(ctx, d, s, contextState, parsed)
		case OutcomeConfirm:
			d.record(ctx, "summary", "confirm")
			return d.advance(ctx, s, valueobjects.PhaseConclusion, "")
		default:
			if s.RepromptCount >= 1 {
				d.record(ctx, "summary", "clarify")
				return d.suspend(ctx, s, valueobjects.PhaseSummary, AwaitingConfirm, d.Interpreter.RenderRetry(ctx, "summary"))
			}
			s.RepromptCount++
			d.record(ctx, "summary", "reprompt")
			return d.suspend(ctx, s, valueobjects.PhaseSummary, AwaitingConfirm, d.Interpreter.RenderRetry(ctx, "summary"))
		}
	})
}

func summaryState(s OnboardingState, contextState OnboardingContext) SummaryState {
	return SummaryState{
		Objective:   contextState.Objective,
		IncomeCents: contextState.IncomeCents,
		Values:      s.Values,
	}
}

func applyCorrection(ctx context.Context, d onboardingDeps, s OnboardingState, _ OnboardingContext, parsed ParsedSummary) (platform.StepOutput[OnboardingState], error) {
	var saveErr error
	switch parsed.Target {
	case CorrectionTargetObjective:
		saveErr = d.ObjectiveSaver.Save(ctx, s.UserID, parsed.NewValue)
	case CorrectionTargetBudget:
		saveErr = applyBudgetCorrection(ctx, d, s.UserID, parsed.NewValue)
	case CorrectionTargetValues:
		saveErr = applyValuesCorrection(ctx, d, &s, parsed.NewValue)
	case CorrectionTargetCards:
		saveErr = applyCardCorrection(ctx, d, s.UserID, parsed.NewValue)
	default:
		return clarifySummary(ctx, d, s)
	}
	if saveErr == errClarify {
		return clarifySummary(ctx, d, s)
	}
	if saveErr != nil {
		return fail(saveErr)
	}
	updatedContext, err := d.ContextLoader.Load(ctx, s.UserID)
	if err != nil {
		return fail(err)
	}
	s.Correction = CorrectionTargetNone
	s.RepromptCount = 0
	d.record(ctx, "summary", "correct")
	return d.suspend(ctx, s, valueobjects.PhaseSummary, AwaitingConfirm, d.Interpreter.RenderSummary(ctx, summaryState(s, updatedContext)))
}

var errClarify = errors.New("clarify")

func clarifySummary(ctx context.Context, d onboardingDeps, s OnboardingState) (platform.StepOutput[OnboardingState], error) {
	d.record(ctx, "summary", "clarify")
	return d.suspend(ctx, s, valueobjects.PhaseSummary, AwaitingConfirm, d.Interpreter.RenderRetry(ctx, "summary"))
}

func applyBudgetCorrection(ctx context.Context, d onboardingDeps, userID uuid.UUID, text string) error {
	cents, ok := ParseMoneyCents(text)
	if !ok || cents <= 0 {
		return errClarify
	}
	return d.IncomeSaver.Save(ctx, userID, cents)
}

func applyValuesCorrection(ctx context.Context, d onboardingDeps, s *OnboardingState, text string) error {
	values, ok := parseValuesCents(text)
	if !ok {
		return errClarify
	}
	applied, saveErr := d.SplitsSaver.Save(ctx, s.UserID, values)
	if saveErr != nil {
		return saveErr
	}
	if !applied {
		return errClarify
	}
	s.Values = values
	return nil
}

func applyCardCorrection(ctx context.Context, d onboardingDeps, userID uuid.UUID, text string) error {
	nickname, dueDay, ok := parseCardInput(text)
	if !ok {
		return errClarify
	}
	return d.CardSaver.Save(ctx, userID, nickname, dueDay)
}

func parseValuesCents(text string) (map[string]int64, bool) {
	if text == "" {
		return nil, false
	}

	tokens := splitValueTokens(text)
	if len(tokens) != len(categoryOrder) {
		return nil, false
	}

	values := make(map[string]int64, len(categoryOrder))
	for i, token := range tokens {
		cents, ok := ParseMoneyCents(token)
		if !ok || cents < 0 {
			return nil, false
		}
		values[categoryOrder[i]] = cents
	}
	return values, true
}

func splitValueTokens(text string) []string {
	if strings.Contains(text, ",") {
		parts := strings.Split(text, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	}
	parts := strings.Fields(text)
	return parts
}

func parseCardInput(text string) (string, int, bool) {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return "", 0, false
	}
	dueDay, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil || dueDay < 1 || dueDay > 31 {
		return "", 0, false
	}
	nickname := strings.Join(fields[:len(fields)-1], " ")
	return nickname, dueDay, nickname != ""
}
