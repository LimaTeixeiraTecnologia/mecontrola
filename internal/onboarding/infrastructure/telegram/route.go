package telegram

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/channels"
	tgdispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/outbound"
	tgpayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/payload"
)

func BuildOnboardingRoute(
	o11y observability.Observability,
	tgCfg configs.TelegramConfig,
	processor *services.TelegramMessageProcessor,
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
		o11y.Logger().Warn(context.Background(), "onboarding.telegram.onboarding_gateway_failed",
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
			o11y.Logger().Warn(ctx, "onboarding.telegram.onboarding_route_failed",
				observability.Error(err),
			)
		}
		if reply == "" {
			return tgdispatcher.OutcomeFallback
		}
		if sendErr := gateway.SendTextMessage(ctx, msg.ChatID, reply); sendErr != nil {
			o11y.Logger().Warn(ctx, "onboarding.telegram.onboarding_reply_failed",
				observability.Error(sendErr),
			)
		}
		return tgdispatcher.OutcomeOnboarding
	}
}
