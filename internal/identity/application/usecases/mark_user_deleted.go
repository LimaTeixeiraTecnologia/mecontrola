package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
)

const prefixMarkUserDeleted = "identity.usecase.mark_user_deleted:"

type MarkUserDeleted struct {
	uow     uow.UnitOfWork[struct{}]
	factory interfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewMarkUserDeleted(
	u uow.UnitOfWork[struct{}],
	factory interfaces.RepositoryFactory,
	o11y observability.Observability,
) *MarkUserDeleted {
	return &MarkUserDeleted{uow: u, factory: factory, o11y: o11y}
}

func (u *MarkUserDeleted) Execute(ctx context.Context, in input.MarkUserDeleted) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.mark_user_deleted")
	defer span.End()

	_, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		userRepo := u.factory.UserRepository(tx)
		if markErr := userRepo.MarkDeleted(ctx, in.ID, time.Now().UTC()); markErr != nil {
			return struct{}{}, fmt.Errorf("%s mark deleted: %w", prefixMarkUserDeleted, markErr)
		}
		return struct{}{}, nil
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
