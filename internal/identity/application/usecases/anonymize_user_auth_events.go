package usecases

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
)

type anonymizeUserPayload struct {
	EventID   string `json:"event_id"`
	UserID    string `json:"user_id"`
	DeletedAt string `json:"deleted_at"`
}

type AnonymizeUserAuthEvents struct {
	factory interfaces.RepositoryFactory
	mgr     manager.Manager
	o11y    observability.Observability
}

func NewAnonymizeUserAuthEvents(
	factory interfaces.RepositoryFactory,
	mgr manager.Manager,
	o11y observability.Observability,
) *AnonymizeUserAuthEvents {
	return &AnonymizeUserAuthEvents{factory: factory, mgr: mgr, o11y: o11y}
}

func (u *AnonymizeUserAuthEvents) Execute(ctx context.Context, in input.AnonymizeUserAuthEvents) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.anonymize_user_auth_events")
	defer span.End()

	var p anonymizeUserPayload
	if err := json.Unmarshal(in.Payload, &p); err != nil {
		u.o11y.Logger().Error(ctx, "identity.usecase.anonymize_user_auth_events.decode_failed",
			observability.Error(err),
		)
		return fmt.Errorf("identity.usecase.anonymize_user_auth_events: decode payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		u.o11y.Logger().Error(ctx, "identity.usecase.anonymize_user_auth_events.invalid_user_id",
			observability.String("raw_user_id", p.UserID),
			observability.Error(err),
		)
		return fmt.Errorf("identity.usecase.anonymize_user_auth_events: parse user_id: %w", err)
	}

	repo := u.factory.AuthEventsRepository(u.mgr.DBTX(ctx))
	if err := repo.AnonymizeByUserID(ctx, userID); err != nil {
		return fmt.Errorf("identity.usecase.anonymize_user_auth_events: anonymize_by_user_id: %w", err)
	}
	return nil
}
