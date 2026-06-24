package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type LoadOnboardingTurns struct {
	repo appinterfaces.OnboardingSessionRepository
	o11y observability.Observability
}

func NewLoadOnboardingTurns(repo appinterfaces.OnboardingSessionRepository, o11y observability.Observability) *LoadOnboardingTurns {
	return &LoadOnboardingTurns{repo: repo, o11y: o11y}
}

func (uc *LoadOnboardingTurns) Execute(ctx context.Context, userID uuid.UUID) ([]entities.OnboardingTurn, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.load_turns")
	defer span.End()

	if userID == uuid.Nil {
		return nil, fmt.Errorf("onboarding: load turns: user id required")
	}

	session, err := uc.repo.Find(ctx, userID)
	if err != nil {
		if errors.Is(err, appinterfaces.ErrOnboardingSessionNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("onboarding: load turns: find session: %w", err)
	}

	turns := session.Payload().RecentTurns
	if turns == nil {
		return []entities.OnboardingTurn{}, nil
	}
	return turns, nil
}
