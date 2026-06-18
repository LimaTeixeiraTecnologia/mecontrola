package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const prefixMarkUserDeleted = "identity.usecase.mark_user_deleted:"

type MarkUserDeleted struct {
	uow       uow.UnitOfWork
	factory   interfaces.RepositoryFactory
	publisher outbox.Publisher
	o11y      observability.Observability
}

func NewMarkUserDeleted(
	u uow.UnitOfWork,
	factory interfaces.RepositoryFactory,
	publisher outbox.Publisher,
	o11y observability.Observability,
) *MarkUserDeleted {
	return &MarkUserDeleted{uow: u, factory: factory, publisher: publisher, o11y: o11y}
}

func (u *MarkUserDeleted) Execute(ctx context.Context, in input.MarkUserDeleted) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.mark_user_deleted")
	defer span.End()

	err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		now := time.Now().UTC()
		userRepo := u.factory.UserRepository(tx)
		if markErr := userRepo.MarkDeleted(ctx, in.ID, now); markErr != nil {
			return fmt.Errorf("%s mark deleted: %w", prefixMarkUserDeleted, markErr)
		}

		ev, buildErr := buildUserDeletedEvent(in.ID, now)
		if buildErr != nil {
			return fmt.Errorf("%s build user.deleted event: %w", prefixMarkUserDeleted, buildErr)
		}
		if pubErr := u.publisher.Publish(ctx, ev); pubErr != nil {
			return fmt.Errorf("%s publish user.deleted: %w", prefixMarkUserDeleted, pubErr)
		}
		return nil
	})

	if err != nil {
		span.RecordError(err)
		u.o11y.Logger().Error(ctx, "identity.usecase.mark_user_deleted_failed",
			observability.String("layer", "usecase"),
			observability.String("operation", "mark_user_deleted"),
			observability.String("user_id", in.ID),
			observability.Error(err),
		)
		return err
	}

	return nil
}

type userDeletedPayload struct {
	EventID   string `json:"event_id"`
	UserID    string `json:"user_id"`
	DeletedAt string `json:"deleted_at"`
}

func buildUserDeletedEvent(userID string, deletedAt time.Time) (outbox.Event, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return outbox.Event{}, fmt.Errorf("generate user.deleted event id: %w", err)
	}
	eventID := id.String()
	payload := userDeletedPayload{
		EventID:   eventID,
		UserID:    userID,
		DeletedAt: deletedAt.Format(time.RFC3339),
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return outbox.Event{}, fmt.Errorf("marshal user.deleted payload: %w", err)
	}
	return outbox.Event{
		ID:              eventID,
		Type:            "user.deleted",
		AggregateType:   "user",
		AggregateID:     userID,
		AggregateUserID: userID,
		Payload:         rawPayload,
		OccurredAt:      deletedAt,
	}, nil
}
