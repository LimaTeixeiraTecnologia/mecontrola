package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

const prefixUpsertUser = "identity.usecase.upsert_user_by_whatsapp:"

type UpsertUserByWhatsApp struct {
	uow     uow.UnitOfWork
	factory interfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewUpsertUserByWhatsApp(
	u uow.UnitOfWork,
	factory interfaces.RepositoryFactory,
	o11y observability.Observability,
) *UpsertUserByWhatsApp {
	return &UpsertUserByWhatsApp{uow: u, factory: factory, o11y: o11y}
}

func (u *UpsertUserByWhatsApp) Execute(ctx context.Context, in input.UpsertUserByWhatsApp) (output.UpsertUserByWhatsApp, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.upsert_user_by_whatsapp")
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.UpsertUserByWhatsApp{}, err
	}

	whatsapp, email, err := u.parseInput(in)
	if err != nil {
		return output.UpsertUserByWhatsApp{}, err
	}

	var result entities.User
	if tx, ok := database.FromContext(ctx); ok {
		result, err = u.persistUpsert(ctx, tx, whatsapp, email, in.DisplayName)
	} else {
		result, err = uow.Do(ctx, u.uow, func(ctx context.Context, tx database.DBTX) (entities.User, error) {
			return u.persistUpsert(ctx, tx, whatsapp, email, in.DisplayName)
		})
	}

	if err != nil {
		if errors.Is(err, application.ErrInvalidWhatsApp) || errors.Is(err, application.ErrInvalidEmail) {
			return output.UpsertUserByWhatsApp{}, err
		}
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

func (u *UpsertUserByWhatsApp) persistUpsert(
	ctx context.Context,
	tx database.DBTX,
	whatsapp valueobjects.WhatsAppNumber,
	email valueobjects.Email,
	displayName string,
) (entities.User, error) {
	userRepo := u.factory.UserRepository(tx)

	var activeFound *entities.User
	existing, findErr := userRepo.FindByWhatsAppNumber(ctx, whatsapp)
	switch {
	case findErr == nil:
		activeFound = &existing
	case errors.Is(findErr, application.ErrUserNotFound):
	default:
		return entities.User{}, fmt.Errorf("%s find by whatsapp: %w", prefixUpsertUser, findErr)
	}

	var deletedFound *entities.User
	if activeFound == nil {
		deleted, findDelErr := userRepo.FindByWhatsAppNumberIncludingDeleted(ctx, whatsapp)
		switch {
		case findDelErr == nil:
			deletedFound = &deleted
		case errors.Is(findDelErr, application.ErrUserNotFound):
		default:
			return entities.User{}, fmt.Errorf("%s find including deleted: %w", prefixUpsertUser, findDelErr)
		}
	}

	now := time.Now().UTC()
	action := services.UserUpsertWorkflow{}.DecideUpsertAction(activeFound, deletedFound, whatsapp, email, displayName, now)

	switch a := action.(type) {
	case services.UpsertUpdateExisting:
		persisted, err := userRepo.UpsertByWhatsAppNumber(ctx, a.Existing, now)
		if err != nil {
			return entities.User{}, fmt.Errorf("%s upsert update: %w", prefixUpsertUser, err)
		}
		return persisted, nil
	case services.UpsertReanimate:
		persisted, err := userRepo.Reanimate(ctx, a.Deleted, now)
		if err != nil {
			return entities.User{}, fmt.Errorf("%s reanimate: %w", prefixUpsertUser, err)
		}
		return persisted, nil
	case services.UpsertCreateNew:
		persisted, err := userRepo.UpsertByWhatsAppNumber(ctx, a.Candidate, now)
		if err != nil {
			return entities.User{}, fmt.Errorf("%s upsert insert: %w", prefixUpsertUser, err)
		}
		return persisted, nil
	default:
		return entities.User{}, fmt.Errorf("%s unknown upsert action", prefixUpsertUser)
	}
}

func (u *UpsertUserByWhatsApp) parseInput(
	in input.UpsertUserByWhatsApp,
) (valueobjects.WhatsAppNumber, valueobjects.Email, error) {
	whatsapp, err := valueobjects.NewWhatsAppNumber(in.WhatsAppNumber)
	if err != nil {
		return valueobjects.WhatsAppNumber{}, valueobjects.Email{}, errors.Join(
			application.ErrInvalidWhatsApp,
			fmt.Errorf("%s parse whatsapp: %w", prefixUpsertUser, err),
		)
	}

	var email valueobjects.Email
	if in.Email == "" {
		return whatsapp, email, nil
	}

	email, err = valueobjects.NewEmail(in.Email)
	if err != nil {
		return valueobjects.WhatsAppNumber{}, valueobjects.Email{}, errors.Join(
			application.ErrInvalidEmail,
			fmt.Errorf("%s parse email: %w", prefixUpsertUser, err),
		)
	}

	return whatsapp, email, nil
}
