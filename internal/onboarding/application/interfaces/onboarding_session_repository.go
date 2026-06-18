package interfaces

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

var ErrOnboardingSessionNotFound = errors.New("onboarding: session not found")

type OnboardingSessionRepository interface {
	Find(ctx context.Context, userID uuid.UUID) (entities.OnboardingSession, error)
	Upsert(ctx context.Context, session entities.OnboardingSession) error
	MarkActive(ctx context.Context, userID uuid.UUID) error
}

type OnboardingSessionRepositoryFactory interface {
	OnboardingSessionRepository(db database.DBTX) OnboardingSessionRepository
}
