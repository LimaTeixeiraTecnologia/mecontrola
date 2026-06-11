package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
)

const prefixProjectAuthEvent = "identity.usecase.project_auth_event:"

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

	authEv, err := parseAuthEvent(in.Payload)
	if err != nil {
		span.RecordError(err)
		u.o11y.Logger().Error(ctx, "identity.usecase.project_auth_event.parse_failed",
			observability.String("event_type", in.EventType),
			observability.Error(err),
		)
		return fmt.Errorf("%s %w", prefixProjectAuthEvent, err)
	}

	repo := u.factory.AuthEventsRepository(u.mgr.DBTX(ctx))
	if err := repo.Insert(ctx, authEv); err != nil {
		return fmt.Errorf("%s insert: %w", prefixProjectAuthEvent, err)
	}
	return nil
}
