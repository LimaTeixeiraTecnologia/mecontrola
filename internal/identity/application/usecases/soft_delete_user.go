package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
)

// SoftDeleteUserUseCase orquestra o soft delete de um User e a cascata em
// user_whatsapp_history (ADR-009). A cascata real é executada no adapter Postgres (task 7.0).
type SoftDeleteUserUseCase struct {
	userRepository interfaces.UserRepository
	o11y           observability.Observability
	metrics        *observability.UsecaseMetrics
}

func NewSoftDeleteUserUseCase(
	userRepository interfaces.UserRepository,
	o11y observability.Observability,
	metrics *observability.UsecaseMetrics,
) *SoftDeleteUserUseCase {
	return &SoftDeleteUserUseCase{
		userRepository: userRepository,
		o11y:           o11y,
		metrics:        metrics,
	}
}

func (u *SoftDeleteUserUseCase) Execute(ctx context.Context, rawID string, now time.Time) error {
	_, err := observability.Observe(ctx, u.o11y, u.metrics, "identity", "soft_delete_user", func(ctx context.Context) (struct{}, error) {
		id, err := entities.NewUserID(rawID)
		if err != nil {
			return struct{}{}, fmt.Errorf("deletar usuário: %w", err)
		}
		if err := u.userRepository.SoftDelete(ctx, id, now); err != nil {
			return struct{}{}, fmt.Errorf("deletar usuário: %w", err)
		}
		return struct{}{}, nil
	})
	return err
}
