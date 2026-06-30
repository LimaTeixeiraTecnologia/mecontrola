package server

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	identityserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/middleware"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/stringsutil"
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wahandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
	wastatus "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/status"
)

func composeWhatsAppWebhookRouter(
	cfg *configs.Config,
	o11y observability.Observability,
	identityModule identity.IdentityModule,
	agentsModule agents.Module,
	onboardingModule onboarding.OnboardingModule,
) *identityserver.WhatsAppWebhookRouter {
	disp := wadispatcher.New(
		identityModule.WhatsAppDedupRepository,
		identityModule.EstablishPrincipal,
		identityModule.WhatsAppLimiter,
		identityModule.OutboxPublisher,
		agentsModule.WhatsAppAgentRoute,
		onboardingModule.WhatsAppActivationRoute,
		o11y,
	)

	verifyHandler := wahandlers.NewVerifyHandler(cfg.WhatsAppConfig.VerifyToken)
	inboundHandler := wahandlers.NewInboundHandler(disp, o11y)

	recordMessageStatus := wastatus.NewRecordMessageStatus(identityModule.WhatsAppMessageStatusRepo, o11y)
	statusHandler := wahandlers.NewStatusHandler(recordMessageStatus, o11y)

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
		statusHandler,
		cfg.WhatsAppConfig.AppSecret,
		cfg.WhatsAppConfig.AppSecretNext,
		waRateLimiter.Middleware,
		func() { waRateLimitExceededTotal.Increment(context.Background()) },
	)
}
