package server

import (
	"context"
	"regexp"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	identityserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wahandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
)

var ativarTokenRegex = regexp.MustCompile(`(?i)^\s*ATIVAR\s+([A-Za-z0-9_\-]{40,45})\s*$`)

func composeWhatsAppWebhookRouter(
	cfg *configs.Config,
	o11y observability.Observability,
	identityModule identity.IdentityModule,
	onboardingModule onboarding.OnboardingModule,
) *identityserver.WhatsAppWebhookRouter {
	agentTemplates := map[string]string{
		"agent_stub_received": cfg.WhatsAppConfig.AgentStubReceived,
	}
	stubAgent := agent.NewStubAgent(onboardingModule.WhatsAppGateway, agentTemplates, o11y)
	processor := onboardingModule.WhatsAppMessageProcessor

	onboardingRoute := func(ctx context.Context, msg payload.Message) wadispatcher.RouteOutcome {
		if matches := ativarTokenRegex.FindStringSubmatch(strings.TrimSpace(msg.Text)); matches != nil {
			if err := processor.HandleActivation(ctx, msg.From, matches[1]); err != nil {
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

	agentRoute := func(ctx context.Context, msg payload.Message) wadispatcher.RouteOutcome {
		if err := stubAgent.HandleMessage(ctx, msg); err != nil {
			o11y.Logger().Warn(ctx, "whatsapp.dispatcher.agent_route_failed",
				observability.Error(err),
			)
		}
		return wadispatcher.OutcomeAgent
	}

	disp := wadispatcher.New(
		identityModule.WhatsAppDedupRepository,
		identityModule.EstablishPrincipal,
		identityModule.WhatsAppLimiter,
		identityModule.OutboxPublisher,
		onboardingRoute,
		agentRoute,
		o11y,
	)

	verifyHandler := wahandlers.NewVerifyHandler(cfg.WhatsAppConfig.VerifyToken)
	inboundHandler := wahandlers.NewInboundHandler(disp, o11y)

	return identityserver.NewWhatsAppWebhookRouter(
		verifyHandler,
		inboundHandler,
		cfg.WhatsAppConfig.AppSecret,
		cfg.WhatsAppConfig.AppSecretNext,
	)
}
