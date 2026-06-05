package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

const prefixUpsertUser = "identity.usecase.upsert_user_by_whatsapp:"

type UpsertUserByWhatsApp struct {
	uow     uow.UnitOfWork[entities.User]
	factory interfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewUpsertUserByWhatsApp(
	u uow.UnitOfWork[entities.User],
	factory interfaces.RepositoryFactory,
	o11y observability.Observability,
) *UpsertUserByWhatsApp {
	return &UpsertUserByWhatsApp{uow: u, factory: factory, o11y: o11y}
}

func (u *UpsertUserByWhatsApp) Execute(ctx context.Context, in input.UpsertUserByWhatsApp) (output.UpsertUserByWhatsApp, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.upsert_user_by_whatsapp")
	defer span.End()

	whatsapp, err := valueobjects.NewWhatsAppNumber(in.WhatsAppNumber)
	if err != nil {
		return output.UpsertUserByWhatsApp{}, fmt.Errorf("%s parse whatsapp: %w", prefixUpsertUser, err)
	}

	var email valueobjects.Email
	if in.Email != "" {
		email, err = valueobjects.NewEmail(in.Email)
		if err != nil {
			return output.UpsertUserByWhatsApp{}, fmt.Errorf("%s parse email: %w", prefixUpsertUser, err)
		}
	}

	result, err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.User, error) {
		userRepo := u.factory.UserRepository(tx)

		existing, findErr := userRepo.FindByWhatsAppNumber(ctx, whatsapp)
		switch {
		case findErr == nil:
			existing.SetDisplayNameIfEmpty(in.DisplayName)
			existing.SetEmailIfEmpty(email)
			persisted, upsertErr := userRepo.UpsertByWhatsAppNumber(ctx, existing, time.Now().UTC())
			if upsertErr != nil {
				return entities.User{}, fmt.Errorf("%s upsert update: %w", prefixUpsertUser, upsertErr)
			}
			return persisted, nil

		case errors.Is(findErr, application.ErrUserNotFound):

		default:
			return entities.User{}, fmt.Errorf("%s find by whatsapp: %w", prefixUpsertUser, findErr)
		}

		deleted, findDelErr := userRepo.FindByWhatsAppNumberIncludingDeleted(ctx, whatsapp)
		switch {
		case findDelErr == nil && deleted.CanReanimate(time.Now().UTC()):
			deleted.Reanimate(time.Now().UTC())
			deleted.SetDisplayNameIfEmpty(in.DisplayName)
			deleted.SetEmailIfEmpty(email)
			persisted, reanErr := userRepo.Reanimate(ctx, deleted, time.Now().UTC())
			if reanErr != nil {
				return entities.User{}, fmt.Errorf("%s reanimate: %w", prefixUpsertUser, reanErr)
			}
			return persisted, nil

		case findDelErr != nil && !errors.Is(findDelErr, application.ErrUserNotFound):
			return entities.User{}, fmt.Errorf("%s find including deleted: %w", prefixUpsertUser, findDelErr)
		}

		candidate := entities.New(whatsapp,
			entities.WithEmail(email),
			entities.WithDisplayName(in.DisplayName),
		)
		persisted, upsertErr := userRepo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
		if upsertErr != nil {
			return entities.User{}, fmt.Errorf("%s upsert insert: %w", prefixUpsertUser, upsertErr)
		}
		return persisted, nil
	})

	if err != nil {
		span.RecordError(err)
		u.o11y.Logger().Error(ctx, "identity.usecase.upsert_failed",
			observability.String("layer", "usecase"),
			observability.String("operation", "upsert_user_by_whatsapp"),
			observability.String("whatsapp", whatsapp.Masked()),
			observability.Error(err),
		)
		return output.UpsertUserByWhatsApp{}, err
	}

	return output.UpsertUserByWhatsApp{
		ID:             result.ID(),
		WhatsAppNumber: result.WhatsApp().String(),
		Email:          result.Email().String(),
		DisplayName:    result.DisplayName(),
		Status:         string(result.Status()),
		CreatedAt:      result.CreatedAt(),
		UpdatedAt:      result.UpdatedAt(),
	}, nil
}
