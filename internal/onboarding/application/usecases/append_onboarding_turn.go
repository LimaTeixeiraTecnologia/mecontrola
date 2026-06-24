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
)

type AppendOnboardingTurnInput struct {
	UserID         uuid.UUID
	UserMessage    string
	AssistantReply string
}

type AppendOnboardingTurn struct {
	uow     uow.UnitOfWork
	factory appinterfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewAppendOnboardingTurn(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	o11y observability.Observability,
) *AppendOnboardingTurn {
	return &AppendOnboardingTurn{uow: u, factory: factory, o11y: o11y}
}

func (uc *AppendOnboardingTurn) Execute(ctx context.Context, in AppendOnboardingTurnInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.append_turn")
	defer span.End()

	if in.UserID == uuid.Nil {
		return fmt.Errorf("onboarding: append turn: user id required")
	}

	_, err := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		repo := uc.factory.OnboardingSessionRepository(tx)
		session, findErr := repo.Find(ctx, in.UserID)
		if findErr != nil {
			if errors.Is(findErr, appinterfaces.ErrOnboardingSessionNotFound) {
				return struct{}{}, findErr
			}
			return struct{}{}, fmt.Errorf("onboarding: append turn: find session: %w", findErr)
		}

		now := time.Now().UTC()
		updated := session.WithAppendedTurn("user", in.UserMessage, now).
			WithAppendedTurn("assistant", in.AssistantReply, now)

		if upsertErr := repo.Upsert(ctx, updated); upsertErr != nil {
			return struct{}{}, fmt.Errorf("onboarding: append turn: upsert session: %w", upsertErr)
		}
		return struct{}{}, nil
	})
	return err
}
