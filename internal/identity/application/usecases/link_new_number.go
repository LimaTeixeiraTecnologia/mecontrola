package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
)

// ErrReservedReason é retornado quando o caller tenta usar a string reservada
// "user_soft_deleted" como reason em LinkNewNumber (ADR-009).
var ErrReservedReason = errors.New("identity: reason 'user_soft_deleted' é reservada para uso interno")

// LinkNewNumberUseCase orquestra a vinculação de um novo número WhatsApp a um User existente,
// registrando o histórico atomicamente (ADR-010).
type LinkNewNumberUseCase struct {
	userRepository interfaces.UserRepository
	o11y           observability.Observability
	metrics        *observability.UsecaseMetrics
}

func NewLinkNewNumberUseCase(
	userRepository interfaces.UserRepository,
	o11y observability.Observability,
	metrics *observability.UsecaseMetrics,
) *LinkNewNumberUseCase {
	return &LinkNewNumberUseCase{
		userRepository: userRepository,
		o11y:           o11y,
		metrics:        metrics,
	}
}

func (u *LinkNewNumberUseCase) Execute(ctx context.Context, rawID, rawNumber, reason string, now time.Time) error {
	_, err := observability.Observe(ctx, u.o11y, u.metrics, "identity", "link_new_number", func(ctx context.Context) (struct{}, error) {
		if reason == "user_soft_deleted" {
			return struct{}{}, fmt.Errorf("vincular novo número: %w", ErrReservedReason)
		}
		id, err := entities.NewUserID(rawID)
		if err != nil {
			return struct{}{}, fmt.Errorf("vincular novo número: %w", err)
		}
		number, err := valueobjects.NewWhatsAppNumber(rawNumber)
		if err != nil {
			return struct{}{}, fmt.Errorf("vincular novo número: %w", err)
		}
		if err := u.userRepository.LinkNewNumber(ctx, id, number, reason, now); err != nil {
			return struct{}{}, fmt.Errorf("vincular novo número: %w", err)
		}
		return struct{}{}, nil
	})
	return err
}
