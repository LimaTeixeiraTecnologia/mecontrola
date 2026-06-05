package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

const prefixFindUserByWhatsApp = "identity.usecase.find_user_by_whatsapp:"

type FindUserByWhatsApp struct {
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
	o11y    observability.Observability
}

func NewFindUserByWhatsApp(
	mgr manager.Manager,
	factory interfaces.RepositoryFactory,
	o11y observability.Observability,
) *FindUserByWhatsApp {
	return &FindUserByWhatsApp{mgr: mgr, factory: factory, o11y: o11y}
}

func (u *FindUserByWhatsApp) Execute(ctx context.Context, in input.FindUserByWhatsApp) (output.FindUser, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.find_user_by_whatsapp")
	defer span.End()

	whatsapp, err := valueobjects.NewWhatsAppNumber(in.WhatsAppNumber)
	if err != nil {
		return output.FindUser{}, fmt.Errorf("%s parse whatsapp: %w", prefixFindUserByWhatsApp, err)
	}

	userRepo := u.factory.UserRepository(u.mgr.DBTX(ctx))
	result, err := userRepo.FindByWhatsAppNumber(ctx, whatsapp)
	if err != nil {
		span.RecordError(err)
		u.o11y.Logger().Error(ctx, "identity.usecase.find_user_by_whatsapp_failed",
			observability.String("layer", "usecase"),
			observability.String("operation", "find_user_by_whatsapp"),
			observability.String("whatsapp", whatsapp.Masked()),
			observability.Error(err),
		)
		return output.FindUser{}, fmt.Errorf("%s find by whatsapp: %w", prefixFindUserByWhatsApp, err)
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
