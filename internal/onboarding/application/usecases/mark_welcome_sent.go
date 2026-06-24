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

type MarkWelcomeSentInput struct {
	UserID uuid.UUID
}

type MarkWelcomeSentResult struct {
	AlreadySent bool
}

type MarkWelcomeSent struct {
	uow     uow.UnitOfWork
	factory appinterfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewMarkWelcomeSent(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	o11y observability.Observability,
) *MarkWelcomeSent {
	return &MarkWelcomeSent{uow: u, factory: factory, o11y: o11y}
}

func (uc *MarkWelcomeSent) Execute(ctx context.Context, in MarkWelcomeSentInput) (MarkWelcomeSentResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.mark_welcome_sent")
	defer span.End()

	if in.UserID == uuid.Nil {
		return MarkWelcomeSentResult{}, fmt.Errorf("onboarding: mark welcome sent: user id required")
	}

	return uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (MarkWelcomeSentResult, error) {
		repo := uc.factory.OnboardingSessionRepository(tx)
		session, findErr := repo.Find(ctx, in.UserID)
		if findErr != nil {
			if errors.Is(findErr, appinterfaces.ErrOnboardingSessionNotFound) {
				return MarkWelcomeSentResult{}, findErr
			}
			return MarkWelcomeSentResult{}, fmt.Errorf("onboarding: mark welcome sent: find session: %w", findErr)
		}

		if session.Payload().WelcomeSentAt != nil {
			return MarkWelcomeSentResult{AlreadySent: true}, nil
		}

		updated := session.WithWelcomeSent(time.Now().UTC())
		if upsertErr := repo.Upsert(ctx, updated); upsertErr != nil {
			return MarkWelcomeSentResult{}, fmt.Errorf("onboarding: mark welcome sent: upsert session: %w", upsertErr)
		}
		return MarkWelcomeSentResult{AlreadySent: false}, nil
	})
}
