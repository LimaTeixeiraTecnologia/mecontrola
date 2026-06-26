package onboarding

import (
	"context"

	"github.com/google/uuid"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbvalueobjects "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type onboardingProgressChecker struct {
	getContext *onbusecases.GetOnboardingContext
}

func NewOnboardingProgressChecker(getContext *onbusecases.GetOnboardingContext) appservices.OnboardingStateChecker {
	if getContext == nil {
		return nil
	}
	return &onboardingProgressChecker{getContext: getContext}
}

func (c *onboardingProgressChecker) Check(ctx context.Context, userID uuid.UUID) (bool, onbvalueobjects.OnboardingPhase, error) {
	out, err := c.getContext.Execute(ctx, onbusecases.GetOnboardingContextInput{UserID: userID})
	if err != nil {
		return false, onbvalueobjects.PhaseWelcome, err
	}
	if !out.Found {
		return false, onbvalueobjects.PhaseWelcome, nil
	}
	if out.CompletedAt != nil {
		return false, onbvalueobjects.PhaseWelcome, nil
	}
	if !out.Phase.IsValid() {
		return true, onbvalueobjects.PhaseWelcome, nil
	}
	return true, out.Phase, nil
}
