package server

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	identityserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/middleware"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/channels"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/stringsutil"
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wahandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
)

func composeWhatsAppWebhookRouter(
	cfg *configs.Config,
	o11y observability.Observability,
	identityModule identity.IdentityModule,
	onboardingModule onboarding.OnboardingModule,
	agentModule agent.AgentModule,
) *identityserver.WhatsAppWebhookRouter {
	processor := onboardingModule.WhatsAppMessageProcessor

	onboardingRoute := func(ctx context.Context, msg payload.Message) wadispatcher.RouteOutcome {
		if token, ok := channels.MatchActivationCommand(msg.Text); ok {
			if err := processor.HandleActivation(ctx, msg.From, token); err != nil {
				o11y.Logger().Warn(ctx, "whatsapp.dispatcher.onboarding_activation_failed",
					observability.Error(err),
				)
			}
			return wadispatcher.OutcomeOnboarding
		}
		if err := processor.HandleFallback(ctx, msg.From); err != nil {
			o11y.Logger().Warn(ctx, "whatsapp.dispatcher.onboarding_fallback_failed",
				observability.Error(err),
			)
		}
		return wadispatcher.OutcomeFallback
	}

	disp := wadispatcher.New(
		identityModule.WhatsAppDedupRepository,
		identityModule.EstablishPrincipal,
		identityModule.WhatsAppLimiter,
		identityModule.OutboxPublisher,
		onboardingRoute,
		agentModule.WhatsAppAgentRoute,
		o11y,
	)

	verifyHandler := wahandlers.NewVerifyHandler(cfg.WhatsAppConfig.VerifyToken)
	inboundHandler := wahandlers.NewInboundHandler(disp, o11y)

	waRateLimiter := middleware.NewRateLimiter(
		cfg.WhatsAppConfig.WebhookRateLimitPerMin,
		cfg.WhatsAppConfig.WebhookRateLimitBurst,
		stringsutil.ParseCSV(cfg.OnboardingConfig.TrustedProxies),
	)

	waRateLimitExceededTotal := o11y.Metrics().Counter(
		"whatsapp_webhook_rate_limit_exceeded_total",
		"Total de requisicoes bloqueadas pelo rate limit do webhook WhatsApp",
		"1",
	)

	return identityserver.NewWhatsAppWebhookRouter(
		verifyHandler,
		inboundHandler,
		cfg.WhatsAppConfig.AppSecret,
		cfg.WhatsAppConfig.AppSecretNext,
		waRateLimiter.Middleware,
		func() { waRateLimitExceededTotal.Increment(context.Background()) },
	)
}
