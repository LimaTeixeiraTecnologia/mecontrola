package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
)

// UpsertUserByWhatsAppNumberUseCase orquestra a criação ou recuperação de um User
// pelo número WhatsApp normalizado.
type UpsertUserByWhatsAppNumberUseCase struct {
	userRepository interfaces.UserRepository
	o11y           observability.Observability
	metrics        *observability.UsecaseMetrics
}

func NewUpsertUserByWhatsAppNumberUseCase(
	userRepository interfaces.UserRepository,
	o11y observability.Observability,
	metrics *observability.UsecaseMetrics,
) *UpsertUserByWhatsAppNumberUseCase {
	return &UpsertUserByWhatsAppNumberUseCase{
		userRepository: userRepository,
		o11y:           o11y,
		metrics:        metrics,
	}
}

func (u *UpsertUserByWhatsAppNumberUseCase) Execute(ctx context.Context, rawNumber string, now time.Time) (*entities.User, error) {
	return observability.Observe(ctx, u.o11y, u.metrics, "identity", "upsert_user_by_whatsapp", func(ctx context.Context) (*entities.User, error) {
		number, err := valueobjects.NewWhatsAppNumber(rawNumber)
		if err != nil {
			return nil, fmt.Errorf("upsert por whatsapp: %w", err)
		}
		user, err := u.userRepository.UpsertByWhatsAppNumber(ctx, number, now)
		if err != nil {
			return nil, fmt.Errorf("upsert por whatsapp: %w", err)
		}
		return user, nil
	})
}
