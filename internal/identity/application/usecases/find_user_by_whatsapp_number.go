package usecases

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
)

// FindUserByWhatsAppNumberUseCase localiza um User ativo pelo número WhatsApp normalizado.
type FindUserByWhatsAppNumberUseCase struct {
	userRepository interfaces.UserRepository
	o11y           observability.Observability
	metrics        *observability.UsecaseMetrics
}

func NewFindUserByWhatsAppNumberUseCase(
	userRepository interfaces.UserRepository,
	o11y observability.Observability,
	metrics *observability.UsecaseMetrics,
) *FindUserByWhatsAppNumberUseCase {
	return &FindUserByWhatsAppNumberUseCase{
		userRepository: userRepository,
		o11y:           o11y,
		metrics:        metrics,
	}
}

func (u *FindUserByWhatsAppNumberUseCase) Execute(ctx context.Context, rawNumber string) (*entities.User, error) {
	return observability.Observe(ctx, u.o11y, u.metrics, "identity", "find_user_by_whatsapp", func(ctx context.Context) (*entities.User, error) {
		number, err := valueobjects.NewWhatsAppNumber(rawNumber)
		if err != nil {
			return nil, fmt.Errorf("buscar usuário por whatsapp: %w", err)
		}
		user, err := u.userRepository.FindByWhatsAppNumber(ctx, number)
		if err != nil {
			return nil, fmt.Errorf("buscar usuário por whatsapp: %w", err)
		}
		return user, nil
	})
}
