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

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type SaveOnboardingCardInput struct {
	UserID   uuid.UUID
	Nickname string
	DueDay   int
}

type SaveOnboardingCardResult struct {
	Name      string
	DueDay    int
	CardCount int
}

type SaveOnboardingCard struct {
	uow       uow.UnitOfWork
	factory   appinterfaces.RepositoryFactory
	publisher outbox.Publisher
	idGen     id.Generator
	o11y      observability.Observability
}

func NewSaveOnboardingCard(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	publisher outbox.Publisher,
	idGen id.Generator,
	o11y observability.Observability,
) *SaveOnboardingCard {
	return &SaveOnboardingCard{uow: u, factory: factory, publisher: publisher, idGen: idGen, o11y: o11y}
}

func (uc *SaveOnboardingCard) Execute(ctx context.Context, in SaveOnboardingCardInput) (SaveOnboardingCardResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.save_card")
	defer span.End()

	if in.UserID == uuid.Nil {
		return SaveOnboardingCardResult{}, fmt.Errorf("onboarding: save card: user id required")
	}

	card, err := entities.NewOnboardingCardDraft(in.Nickname, in.DueDay)
	if err != nil {
		return SaveOnboardingCardResult{}, err
	}

	return uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (SaveOnboardingCardResult, error) {
		repo := uc.factory.OnboardingSessionRepository(tx)
		session, findErr := repo.Find(ctx, in.UserID)
		if findErr != nil {
			if errors.Is(findErr, appinterfaces.ErrOnboardingSessionNotFound) {
				return SaveOnboardingCardResult{}, findErr
			}
			return SaveOnboardingCardResult{}, fmt.Errorf("onboarding: save card: find session: %w", findErr)
		}

		now := time.Now().UTC()
		updated := session.WithAppendedCard(card, now)
		if upsertErr := repo.Upsert(ctx, updated); upsertErr != nil {
			return SaveOnboardingCardResult{}, fmt.Errorf("onboarding: save card: upsert session: %w", upsertErr)
		}

		event := entities.CardRegistered{
			EventID:    newEventID(uc.idGen),
			UserID:     in.UserID,
			Channel:    session.Channel().String(),
			Name:       card.Name,
			LimitCents: card.LimitCents,
			ClosingDay: card.ClosingDay,
			DueDay:     card.DueDay,
			OccurredAt: now,
		}
		envelope, buildErr := buildOutboxEvent(in.UserID, event, now)
		if buildErr != nil {
			return SaveOnboardingCardResult{}, fmt.Errorf("onboarding: save card: build event: %w", buildErr)
		}
		if pubErr := uc.publisher.Publish(ctx, envelope); pubErr != nil {
			return SaveOnboardingCardResult{}, fmt.Errorf("onboarding: save card: publish event: %w", pubErr)
		}
		return SaveOnboardingCardResult{
			Name:      card.Name,
			DueDay:    card.DueDay,
			CardCount: len(updated.Payload().Cards),
		}, nil
	})
}
