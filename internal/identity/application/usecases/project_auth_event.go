package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type projectAuthEventPayload struct {
	EventID    string  `json:"event_id"`
	UserID     *string `json:"user_id"`
	Kind       string  `json:"kind"`
	Source     string  `json:"source"`
	Reason     *string `json:"reason"`
	OccurredAt string  `json:"occurred_at"`
}

type ProjectAuthEvent struct {
	factory interfaces.RepositoryFactory
	mgr     manager.Manager
	o11y    observability.Observability
}

func NewProjectAuthEvent(
	factory interfaces.RepositoryFactory,
	mgr manager.Manager,
	o11y observability.Observability,
) *ProjectAuthEvent {
	return &ProjectAuthEvent{factory: factory, mgr: mgr, o11y: o11y}
}

func (u *ProjectAuthEvent) Execute(ctx context.Context, in input.ProjectAuthEvent) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.project_auth_event")
	defer span.End()

	var p projectAuthEventPayload
	if err := json.Unmarshal(in.Payload, &p); err != nil {
		u.o11y.Logger().Error(ctx, "identity.usecase.project_auth_event.decode_failed",
			observability.String("event_type", in.EventType),
			observability.Error(err),
		)
		return fmt.Errorf("identity.usecase.project_auth_event: decode payload: %w", err)
	}

	eventID, err := uuid.Parse(p.EventID)
	if err != nil {
		u.o11y.Logger().Error(ctx, "identity.usecase.project_auth_event.invalid_event_id",
			observability.String("event_type", in.EventType),
			observability.String("raw_event_id", p.EventID),
			observability.Error(err),
		)
		return fmt.Errorf("identity.usecase.project_auth_event: parse event_id: %w", err)
	}

	occurredAt, err := time.Parse(time.RFC3339, p.OccurredAt)
	if err != nil {
		u.o11y.Logger().Error(ctx, "identity.usecase.project_auth_event.invalid_occurred_at",
			observability.String("event_type", in.EventType),
			observability.String("event_id", p.EventID),
			observability.Error(err),
		)
		return fmt.Errorf("identity.usecase.project_auth_event: parse occurred_at: %w", err)
	}

	var userID *uuid.UUID
	if p.UserID != nil {
		uid, parseErr := uuid.Parse(*p.UserID)
		if parseErr != nil {
			u.o11y.Logger().Error(ctx, "identity.usecase.project_auth_event.invalid_user_id",
				observability.String("event_type", in.EventType),
				observability.String("event_id", p.EventID),
				observability.Error(parseErr),
			)
			return fmt.Errorf("identity.usecase.project_auth_event: parse user_id: %w", parseErr)
		}
		userID = &uid
	}

	var reason *entities.AuthEventReason
	if p.Reason != nil {
		r := entities.AuthEventReason(*p.Reason)
		reason = &r
	}

	authEv := entities.HydrateAuthEvent(
		eventID,
		occurredAt,
		userID,
		entities.AuthEventKind(p.Kind),
		entities.AuthEventSource(p.Source),
		reason,
	)

	repo := u.factory.AuthEventsRepository(u.mgr.DBTX(ctx))
	if err := repo.Insert(ctx, authEv); err != nil {
		return fmt.Errorf("identity.usecase.project_auth_event: insert: %w", err)
	}
	return nil
}
