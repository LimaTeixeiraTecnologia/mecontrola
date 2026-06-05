package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

const prefixFindUserByID = "identity.usecase.find_user_by_id:"

type FindUserByID struct {
	uow     uow.UnitOfWork[entities.User]
	factory interfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewFindUserByID(
	u uow.UnitOfWork[entities.User],
	factory interfaces.RepositoryFactory,
	o11y observability.Observability,
) *FindUserByID {
	return &FindUserByID{uow: u, factory: factory, o11y: o11y}
}

func (u *FindUserByID) Execute(ctx context.Context, in input.FindUserByID) (output.FindUser, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.find_user_by_id")
	defer span.End()

	result, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.User, error) {
		userRepo := u.factory.UserRepository(tx)

		found, findErr := userRepo.FindByID(ctx, in.ID)
		if findErr != nil {
			return entities.User{}, fmt.Errorf("%s find by id: %w", prefixFindUserByID, findErr)
		}
		return found, nil
	})

	if err != nil {
		u.o11y.Logger().Error(ctx, "identity.usecase.find_user_by_id_failed",
			observability.String("layer", "usecase"),
			observability.String("operation", "find_user_by_id"),
			observability.String("user_id", in.ID),
			observability.Error(err),
		)
		return output.FindUser{}, err
	}

	return output.FindUser{
		ID:             result.ID(),
		WhatsAppNumber: result.WhatsApp().String(),
		Email:          result.Email().String(),
		DisplayName:    result.DisplayName(),
		Status:         string(result.Status()),
		CreatedAt:      result.CreatedAt(),
		UpdatedAt:      result.UpdatedAt(),
	}, nil
}
