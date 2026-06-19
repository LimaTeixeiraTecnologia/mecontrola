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

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type SaveOnboardingIncomeInput struct {
	UserID      uuid.UUID
	IncomeCents int64
}

type SaveOnboardingIncomeResult struct {
	IncomeCents int64
}

type SaveOnboardingIncome struct {
	uow       uow.UnitOfWork
	factory   appinterfaces.RepositoryFactory
	publisher outbox.Publisher
	idGen     id.Generator
	o11y      observability.Observability
}

func NewSaveOnboardingIncome(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	publisher outbox.Publisher,
	idGen id.Generator,
	o11y observability.Observability,
) *SaveOnboardingIncome {
	return &SaveOnboardingIncome{uow: u, factory: factory, publisher: publisher, idGen: idGen, o11y: o11y}
}

func (uc *SaveOnboardingIncome) Execute(ctx context.Context, in SaveOnboardingIncomeInput) (SaveOnboardingIncomeResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.save_income")
	defer span.End()

	if in.UserID == uuid.Nil {
		return SaveOnboardingIncomeResult{}, fmt.Errorf("onboarding: save income: user id required")
	}

	income, err := valueobjects.NewMonthlyIncome(in.IncomeCents)
	if err != nil {
		return SaveOnboardingIncomeResult{}, err
	}

	return uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (SaveOnboardingIncomeResult, error) {
		repo := uc.factory.OnboardingSessionRepository(tx)
		session, findErr := repo.Find(ctx, in.UserID)
		if findErr != nil {
			if errors.Is(findErr, appinterfaces.ErrOnboardingSessionNotFound) {
				return SaveOnboardingIncomeResult{}, findErr
			}
			return SaveOnboardingIncomeResult{}, fmt.Errorf("onboarding: save income: find session: %w", findErr)
		}

		now := time.Now().UTC()
		updated := session.WithIncome(income, now)
		if upsertErr := repo.Upsert(ctx, updated); upsertErr != nil {
			return SaveOnboardingIncomeResult{}, fmt.Errorf("onboarding: save income: upsert session: %w", upsertErr)
		}

		event := entities.IncomeRegistered{
			EventID:     newEventID(uc.idGen),
			UserID:      in.UserID,
			Channel:     session.Channel().String(),
			IncomeCents: income.Cents(),
			OccurredAt:  now,
		}
		envelope, buildErr := buildOutboxEvent(in.UserID, event, now)
		if buildErr != nil {
			return SaveOnboardingIncomeResult{}, fmt.Errorf("onboarding: save income: build event: %w", buildErr)
		}
		if pubErr := uc.publisher.Publish(ctx, envelope); pubErr != nil {
			return SaveOnboardingIncomeResult{}, fmt.Errorf("onboarding: save income: publish event: %w", pubErr)
		}
		return SaveOnboardingIncomeResult{IncomeCents: income.Cents()}, nil
	})
}
