package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
)

const prefixFindUserByID = "identity.usecase.find_user_by_id:"

type FindUserByID struct {
	repo interfaces.UserRepository
	o11y observability.Observability
}

func NewFindUserByID(
	repo interfaces.UserRepository,
	o11y observability.Observability,
) *FindUserByID {
	return &FindUserByID{repo: repo, o11y: o11y}
}

func (u *FindUserByID) Execute(ctx context.Context, in input.FindUserByID) (output.FindUser, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "identity.usecase.find_user_by_id")
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.FindUser{}, err
	}

	result, err := u.repo.FindByID(ctx, in.ID)
	if err != nil {
		span.RecordError(err)
		u.o11y.Logger().Error(ctx, "identity.usecase.find_user_by_id_failed",
			observability.String("layer", "usecase"),
			observability.String("operation", "find_user_by_id"),
			observability.String("user_id", in.ID),
			observability.Error(err),
		)
		return output.FindUser{}, fmt.Errorf("%s find by id: %w", prefixFindUserByID, err)
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
