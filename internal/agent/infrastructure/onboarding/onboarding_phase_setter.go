package onboarding

import (
	"context"

	"github.com/google/uuid"

	appusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
)

type onboardingPhaseSetter struct {
	setPhase *onbusecases.SetOnboardingPhase
}

func NewOnboardingPhaseSetter(setPhase *onbusecases.SetOnboardingPhase) appusecases.OnboardingPhaseSetter {
	if setPhase == nil {
		return nil
	}
	return &onboardingPhaseSetter{setPhase: setPhase}
}

func (s *onboardingPhaseSetter) SetPhase(ctx context.Context, userID uuid.UUID, phase string) error {
	_, err := s.setPhase.Execute(ctx, onbusecases.SetOnboardingPhaseInput{UserID: userID, Phase: phase})
	return err
}
