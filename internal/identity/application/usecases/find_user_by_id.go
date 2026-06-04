package usecases

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
)

// FindUserByIDUseCase localiza um User ativo pelo seu identificador único.
type FindUserByIDUseCase struct {
	userRepository interfaces.UserRepository
	o11y           observability.Observability
	metrics        *observability.UsecaseMetrics
}

func NewFindUserByIDUseCase(
	userRepository interfaces.UserRepository,
	o11y observability.Observability,
	metrics *observability.UsecaseMetrics,
) *FindUserByIDUseCase {
	return &FindUserByIDUseCase{
		userRepository: userRepository,
		o11y:           o11y,
		metrics:        metrics,
	}
}

func (u *FindUserByIDUseCase) Execute(ctx context.Context, rawID string) (*entities.User, error) {
	return observability.Observe(ctx, u.o11y, u.metrics, "identity", "find_user_by_id", func(ctx context.Context) (*entities.User, error) {
		id, err := entities.NewUserID(rawID)
		if err != nil {
			return nil, fmt.Errorf("buscar usuário por id: %w", err)
		}
		user, err := u.userRepository.FindByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("buscar usuário por id: %w", err)
		}
		return user, nil
	})
}
