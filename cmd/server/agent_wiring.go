package server

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	onboardingapp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	onboardinginterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	onboardingservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/channels"
	tgdispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/outbound"
	tgpayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/payload"
)

type budgetConfiguratorAdapter struct {
	uc *usecases.StartBudgetConfiguration
}

func newBudgetConfiguratorAdapter(uc *usecases.StartBudgetConfiguration) appservices.BudgetConfigurator {
	if uc == nil {
		return nil
	}
	return &budgetConfiguratorAdapter{uc: uc}
}

func (a *budgetConfiguratorAdapter) Start(ctx context.Context, userID uuid.UUID, channel string) (string, error) {
	parsed := mapAgentChannelToOnboarding(channel)
	result, err := a.uc.Execute(ctx, usecases.StartBudgetConfigurationInput{
		UserID:  userID,
		Channel: parsed,
	})
	if err != nil {
		return "", err
	}
	return result.Reply, nil
}

func mapAgentChannelToOnboarding(channel string) entities.OnboardingChannel {
	switch channel {
	case appservices.ChannelTelegram:
		return entities.OnboardingChannelTelegram
	default:
		return entities.OnboardingChannelWhatsApp
	}
}

type onboardingContinuationAdapter struct {
	whatsApp *onboardingservices.WhatsAppMessageProcessor
	telegram *onboardingservices.TelegramMessageProcessor
}

func newOnboardingContinuationAdapter(
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
		return "", fmt.Errorf("agent.wiring: telegram message id invalido: %w", err)
	}
	return trimmed, nil
}

func buildTelegramOnboardingRoute(
	o11y observability.Observability,
	tgCfg configs.TelegramConfig,
	processor *onboardingservices.TelegramMessageProcessor,
) tgdispatcher.OnboardingRoute {
	if processor == nil {
		return nil
	}
	gateway, err := outbound.NewSharedGateway(o11y, outbound.FactoryConfig{
		APIBaseURL: tgCfg.APIBaseURL,
		BotToken:   tgCfg.BotToken,
		Timeout:    tgCfg.OutboundTimeout,
	})
	if err != nil {
		o11y.Logger().Warn(context.Background(), "agent.wiring.telegram_onboarding_gateway_failed",
			observability.Error(err),
		)
		return nil
	}

	return func(ctx context.Context, msg tgpayload.Message) tgdispatcher.RouteOutcome {
		var reply string
		var err error
		if token, ok := channels.MatchActivationCommand(msg.Text); ok {
			reply, err = processor.HandleActivation(ctx, msg.FromUserID, token)
		} else {
			reply, err = processor.HandleFallback(ctx, msg.FromUserID)
		}
		if err != nil {
			o11y.Logger().Warn(ctx, "telegram.dispatcher.onboarding_route_failed",
				observability.Error(err),
			)
		}
		if reply == "" {
			return tgdispatcher.OutcomeFallback
		}
		if sendErr := gateway.SendTextMessage(ctx, msg.ChatID, reply); sendErr != nil {
			o11y.Logger().Warn(ctx, "telegram.dispatcher.onboarding_reply_failed",
				observability.Error(sendErr),
			)
		}
		return tgdispatcher.OutcomeOnboarding
	}
}
