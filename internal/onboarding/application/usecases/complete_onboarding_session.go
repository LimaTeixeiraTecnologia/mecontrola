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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type CompleteOnboardingSessionInput struct {
	UserID uuid.UUID
}

type CompleteOnboardingSessionResult struct {
	Completed     bool
	AlreadyActive bool
}

type CompleteOnboardingSession struct {
	uow       uow.UnitOfWork
	factory   appinterfaces.RepositoryFactory
	publisher outbox.Publisher
	idGen     id.Generator
	o11y      observability.Observability
}

func NewCompleteOnboardingSession(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	publisher outbox.Publisher,
	idGen id.Generator,
	o11y observability.Observability,
) *CompleteOnboardingSession {
	return &CompleteOnboardingSession{uow: u, factory: factory, publisher: publisher, idGen: idGen, o11y: o11y}
}

func (uc *CompleteOnboardingSession) Execute(ctx context.Context, in CompleteOnboardingSessionInput) (CompleteOnboardingSessionResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.complete_session")
	defer span.End()

	if in.UserID == uuid.Nil {
		return CompleteOnboardingSessionResult{}, fmt.Errorf("onboarding: complete session: user id required")
	}

	return uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (CompleteOnboardingSessionResult, error) {
		repo := uc.factory.OnboardingSessionRepository(tx)
		session, findErr := repo.Find(ctx, in.UserID)
		if findErr != nil {
			if errors.Is(findErr, appinterfaces.ErrOnboardingSessionNotFound) {
				return CompleteOnboardingSessionResult{}, findErr
			}
			return CompleteOnboardingSessionResult{}, fmt.Errorf("onboarding: complete session: find session: %w", findErr)
		}

		if session.IsActive() {
			return CompleteOnboardingSessionResult{AlreadyActive: true}, nil
		}
		if !session.IsReadyToComplete() {
			return CompleteOnboardingSessionResult{}, fmt.Errorf("onboarding: complete session: %w", application.ErrOnboardingNotReadyToComplete)
		}

		now := time.Now().UTC()
		completed := session.WithCompletion(now)
		if upsertErr := repo.Upsert(ctx, completed); upsertErr != nil {
			return CompleteOnboardingSessionResult{}, fmt.Errorf("onboarding: complete session: upsert session: %w", upsertErr)
		}

		event := entities.OnboardingCompleted{
			EventID:    newEventID(uc.idGen),
			UserID:     in.UserID,
			Channel:    session.Channel().String(),
			OccurredAt: now,
		}
		envelope, buildErr := buildOutboxEvent(in.UserID, event, now)
		if buildErr != nil {
			return CompleteOnboardingSessionResult{}, fmt.Errorf("onboarding: complete session: build event: %w", buildErr)
		}
		if pubErr := uc.publisher.Publish(ctx, envelope); pubErr != nil {
			return CompleteOnboardingSessionResult{}, fmt.Errorf("onboarding: complete session: publish event: %w", pubErr)
		}
		return CompleteOnboardingSessionResult{Completed: true}, nil
	})
}
