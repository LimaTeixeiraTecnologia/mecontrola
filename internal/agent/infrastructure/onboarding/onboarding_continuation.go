package onboarding

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	onboardingapp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	onboardinginterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	onboardingservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/services"
)

type onboardingContinuationAdapter struct {
	whatsApp *onboardingservices.WhatsAppMessageProcessor
	telegram *onboardingservices.TelegramMessageProcessor
}

func NewOnboardingContinuationAdapter(
	whatsApp *onboardingservices.WhatsAppMessageProcessor,
	telegram *onboardingservices.TelegramMessageProcessor,
) appservices.OnboardingContinuation {
	if whatsApp == nil && telegram == nil {
		return nil
	}
	return &onboardingContinuationAdapter{
		whatsApp: whatsApp,
		telegram: telegram,
	}
}

func (a *onboardingContinuationAdapter) Continue(
	ctx context.Context,
	userID uuid.UUID,
	channel string,
	peer string,
	text string,
	messageID string,
) (appservices.OnboardingConversation, error) {
	switch channel {
	case appservices.ChannelTelegram:
		if a.telegram == nil {
			return appservices.OnboardingConversation{}, nil
		}
		parsedMessageID, err := parseTelegramMessageID(messageID)
		if err != nil {
			return appservices.OnboardingConversation{}, err
		}
		reply, err := a.telegram.ProcessConversation(ctx, userID, text, parsedMessageID)
		if err != nil {
			if errors.Is(err, onboardinginterfaces.ErrOnboardingSessionNotFound) ||
				errors.Is(err, onboardingapp.ErrOnboardingAlreadyActive) {
				return appservices.OnboardingConversation{}, nil
			}
			return appservices.OnboardingConversation{}, err
		}
		return appservices.OnboardingConversation{Handled: true, Reply: reply}, nil
	default:
		if a.whatsApp == nil {
			return appservices.OnboardingConversation{}, nil
		}
		if err := a.whatsApp.ProcessConversation(ctx, userID, peer, text, messageID); err != nil {
			if errors.Is(err, onboardinginterfaces.ErrOnboardingSessionNotFound) ||
				errors.Is(err, onboardingapp.ErrOnboardingAlreadyActive) {
				return appservices.OnboardingConversation{}, nil
			}
			return appservices.OnboardingConversation{}, err
		}
		return appservices.OnboardingConversation{Handled: true}, nil
	}
}

func parseTelegramMessageID(raw string) (string, error) {
	trimmed := raw
	if trimmed == "" {
		return "", nil
	}
	if _, err := strconv.ParseInt(trimmed, 10, 64); err != nil {
		return "", fmt.Errorf("agent.onboarding: telegram message id invalido: %w", err)
	}
	return trimmed, nil
}
