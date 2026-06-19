package onboarding

import (
	"context"

	"github.com/google/uuid"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	appusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
)

type onboardingTurnRunnerAdapter struct {
	uc *appusecases.RunOnboardingTurn
}

func NewOnboardingTurnRunnerAdapter(uc *appusecases.RunOnboardingTurn) appservices.OnboardingTurnRunner {
	if uc == nil {
		return nil
	}
	return &onboardingTurnRunnerAdapter{uc: uc}
}

func (a *onboardingTurnRunnerAdapter) Run(ctx context.Context, userID uuid.UUID, channel, text string) (appservices.OnboardingTurnResult, error) {
	out, err := a.uc.Execute(ctx, appusecases.RunOnboardingTurnInput{
		UserID:  userID,
		Channel: channel,
		Text:    text,
	})
	if err != nil {
		return appservices.OnboardingTurnResult{}, err
	}
	return appservices.OnboardingTurnResult{Handled: out.Handled, Reply: out.Reply}, nil
}
