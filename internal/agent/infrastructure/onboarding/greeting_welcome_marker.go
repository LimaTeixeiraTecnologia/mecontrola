package onboarding

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
)

type GreetingWelcomeMarker struct {
	markWelcomeSent markWelcomeSentUseCase
}

func NewGreetingWelcomeMarker(uc markWelcomeSentUseCase) *GreetingWelcomeMarker {
	return &GreetingWelcomeMarker{markWelcomeSent: uc}
}

func (m *GreetingWelcomeMarker) MarkWelcomeSent(ctx context.Context, userID uuid.UUID) (bool, error) {
	result, err := m.markWelcomeSent.Execute(ctx, onbusecases.MarkWelcomeSentInput{UserID: userID})
	if err != nil {
		return false, fmt.Errorf("greeting_welcome_marker: mark welcome sent: %w", err)
	}
	return result.AlreadySent, nil
}
