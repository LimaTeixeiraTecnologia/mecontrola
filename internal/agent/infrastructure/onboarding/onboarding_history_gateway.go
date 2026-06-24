package onboarding

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onbentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type appendTurnUseCase interface {
	Execute(ctx context.Context, in onbusecases.AppendOnboardingTurnInput) error
}

type loadTurnsUseCase interface {
	Execute(ctx context.Context, userID uuid.UUID) ([]onbentities.OnboardingTurn, error)
}

type markWelcomeSentUseCase interface {
	Execute(ctx context.Context, in onbusecases.MarkWelcomeSentInput) (onbusecases.MarkWelcomeSentResult, error)
}

type OnboardingHistoryGateway struct {
	appendTurn      appendTurnUseCase
	loadTurns       loadTurnsUseCase
	markWelcomeSent markWelcomeSentUseCase
}

func NewOnboardingHistoryGateway(
	appendTurn appendTurnUseCase,
	loadTurns loadTurnsUseCase,
	markWelcomeSent markWelcomeSentUseCase,
) *OnboardingHistoryGateway {
	return &OnboardingHistoryGateway{
		appendTurn:      appendTurn,
		loadTurns:       loadTurns,
		markWelcomeSent: markWelcomeSent,
	}
}

func (g *OnboardingHistoryGateway) LoadTurns(ctx context.Context, userID uuid.UUID) ([]onbentities.OnboardingTurn, error) {
	turns, err := g.loadTurns.Execute(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("agent.onboarding_history_gateway: load turns: %w", err)
	}
	return turns, nil
}

func (g *OnboardingHistoryGateway) AppendTurn(ctx context.Context, userID uuid.UUID, userMsg, assistantReply string) error {
	if err := g.appendTurn.Execute(ctx, onbusecases.AppendOnboardingTurnInput{
		UserID:         userID,
		UserMessage:    userMsg,
		AssistantReply: assistantReply,
	}); err != nil {
		return fmt.Errorf("agent.onboarding_history_gateway: append turn: %w", err)
	}
	return nil
}

func (g *OnboardingHistoryGateway) MarkWelcomeSent(ctx context.Context, userID uuid.UUID) (bool, error) {
	result, err := g.markWelcomeSent.Execute(ctx, onbusecases.MarkWelcomeSentInput{UserID: userID})
	if err != nil {
		return false, fmt.Errorf("agent.onboarding_history_gateway: mark welcome sent: %w", err)
	}
	return result.AlreadySent, nil
}
