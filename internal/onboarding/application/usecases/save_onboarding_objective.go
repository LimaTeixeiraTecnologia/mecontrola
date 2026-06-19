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

type SaveOnboardingObjectiveInput struct {
	UserID    uuid.UUID
	Objective string
}

type SaveOnboardingObjectiveResult struct {
	Objective string
}

type SaveOnboardingObjective struct {
	uow     uow.UnitOfWork
	factory appinterfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewSaveOnboardingObjective(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	o11y observability.Observability,
) *SaveOnboardingObjective {
	return &SaveOnboardingObjective{uow: u, factory: factory, o11y: o11y}
}

func (uc *SaveOnboardingObjective) Execute(ctx context.Context, in SaveOnboardingObjectiveInput) (SaveOnboardingObjectiveResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.save_objective")
	defer span.End()

	if in.UserID == uuid.Nil {
		return SaveOnboardingObjectiveResult{}, fmt.Errorf("onboarding: save objective: user id required")
	}

	objective, err := valueobjects.NewFinancialObjective(in.Objective)
	if err != nil {
		return SaveOnboardingObjectiveResult{}, err
	}

	return uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (SaveOnboardingObjectiveResult, error) {
		repo := uc.factory.OnboardingSessionRepository(tx)
		session, findErr := repo.Find(ctx, in.UserID)
		if findErr != nil {
			if errors.Is(findErr, appinterfaces.ErrOnboardingSessionNotFound) {
				return SaveOnboardingObjectiveResult{}, findErr
			}
			return SaveOnboardingObjectiveResult{}, fmt.Errorf("onboarding: save objective: find session: %w", findErr)
		}
		updated := session.WithObjective(objective, time.Now().UTC())
		if upsertErr := repo.Upsert(ctx, updated); upsertErr != nil {
			return SaveOnboardingObjectiveResult{}, fmt.Errorf("onboarding: save objective: upsert session: %w", upsertErr)
		}
		return SaveOnboardingObjectiveResult{Objective: objective.String()}, nil
	})
}
