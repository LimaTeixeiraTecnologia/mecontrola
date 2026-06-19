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

type MarkFirstTransactionRecordedInput struct {
	UserID uuid.UUID
}

type MarkFirstTransactionRecordedResult struct {
	Marked bool
}

type MarkFirstTransactionRecorded struct {
	uow     uow.UnitOfWork
	factory appinterfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewMarkFirstTransactionRecorded(
	u uow.UnitOfWork,
	factory appinterfaces.RepositoryFactory,
	o11y observability.Observability,
) *MarkFirstTransactionRecorded {
	return &MarkFirstTransactionRecorded{uow: u, factory: factory, o11y: o11y}
}

func (uc *MarkFirstTransactionRecorded) Execute(ctx context.Context, in MarkFirstTransactionRecordedInput) (MarkFirstTransactionRecordedResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.mark_first_transaction")
	defer span.End()

	if in.UserID == uuid.Nil {
		return MarkFirstTransactionRecordedResult{}, fmt.Errorf("onboarding: mark first transaction: user id required")
	}

	return uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (MarkFirstTransactionRecordedResult, error) {
		repo := uc.factory.OnboardingSessionRepository(tx)
		session, findErr := repo.Find(ctx, in.UserID)
		if findErr != nil {
			if errors.Is(findErr, appinterfaces.ErrOnboardingSessionNotFound) {
				return MarkFirstTransactionRecordedResult{Marked: false}, nil
			}
			return MarkFirstTransactionRecordedResult{}, fmt.Errorf("onboarding: mark first transaction: find session: %w", findErr)
		}

		if session.IsActive() {
			return MarkFirstTransactionRecordedResult{Marked: false}, nil
		}
		if session.HasFirstTransaction() {
			return MarkFirstTransactionRecordedResult{Marked: true}, nil
		}

		updated := session.WithFirstTransactionRecorded(time.Now().UTC())
		if upsertErr := repo.Upsert(ctx, updated); upsertErr != nil {
			return MarkFirstTransactionRecordedResult{}, fmt.Errorf("onboarding: mark first transaction: upsert session: %w", upsertErr)
		}
		return MarkFirstTransactionRecordedResult{Marked: true}, nil
	})
}
