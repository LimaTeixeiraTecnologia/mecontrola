package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type SetOnboardingPhaseInput struct {
	UserID uuid.UUID
	Phase  valueobjects.OnboardingPhase
}

type SetOnboardingPhaseResult struct {
	Phase valueobjects.OnboardingPhase
}

type SetOnboardingPhase struct {
	uow     uow.UnitOfWork
	factory appinterfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewSetOnboardingPhase(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	o11y observability.Observability,
) *SetOnboardingPhase {
	return &SetOnboardingPhase{uow: u, factory: factory, o11y: o11y}
}

func (uc *SetOnboardingPhase) Execute(ctx context.Context, in SetOnboardingPhaseInput) (SetOnboardingPhaseResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.set_phase")
	defer span.End()

	if in.UserID == uuid.Nil {
		return SetOnboardingPhaseResult{}, fmt.Errorf("onboarding: set phase: user id required")
	}
	if !in.Phase.IsValid() {
		return SetOnboardingPhaseResult{}, fmt.Errorf("onboarding: set phase: %w", valueobjects.ErrOnboardingPhaseInvalid)
	}

	return uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (SetOnboardingPhaseResult, error) {
		repo := uc.factory.OnboardingSessionRepository(tx)
		session, findErr := repo.Find(ctx, in.UserID)
		if findErr != nil {
			if errors.Is(findErr, appinterfaces.ErrOnboardingSessionNotFound) {
				return SetOnboardingPhaseResult{}, findErr
			}
			return SetOnboardingPhaseResult{}, fmt.Errorf("onboarding: set phase: find session: %w", findErr)
		}
		updated := session.WithPhase(in.Phase, time.Now().UTC())
		if upsertErr := repo.Upsert(ctx, updated); upsertErr != nil {
			return SetOnboardingPhaseResult{}, fmt.Errorf("onboarding: set phase: upsert session: %w", upsertErr)
		}
		return SetOnboardingPhaseResult{Phase: in.Phase}, nil
	})
}
