package server

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	identityserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/middleware"
	onboardingtelegram "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/telegram"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/stringsutil"
	tgdedup "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dedup/postgres"
	tgdispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dispatcher"
	tghandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/handlers"
)

func composeTelegramWebhookRouter(
	cfg *configs.Config,
	o11y observability.Observability,
	db *sqlx.DB,
	identityModule identity.IdentityModule,
	onboardingModule onboarding.OnboardingModule,
	agentModule agent.AgentModule,
) (*identityserver.TelegramWebhookRouter, error) {
	if !cfg.TelegramConfig.Enabled {
		return nil, nil
	}

	onboardingRoute := onboardingtelegram.BuildOnboardingRoute(o11y, cfg.TelegramConfig, onboardingModule.TelegramMessageProcessor)
	dedupRepo := tgdedup.NewUpdateRepository(o11y, db)

	dispatcher := tgdispatcher.New(
		cfg.TelegramConfig.BotID,
		dedupRepo,
		identityModule.ResolvePrincipalByIdentity,
		identityModule.WhatsAppLimiter,
		identityModule.OutboxPublisher,
		onboardingRoute,
		agentModule.TelegramAgentRoute,
		o11y,
	)

	inboundHandler := tghandlers.NewInboundHandler(dispatcher, o11y)
	rateLimiter := middleware.NewRateLimiter(
		cfg.TelegramConfig.WebhookRateLimitPerMin,
		cfg.TelegramConfig.WebhookRateLimitBurst,
		stringsutil.ParseCSV(cfg.OnboardingConfig.TrustedProxies),
	)
	rateLimitExceededTotal := o11y.Metrics().Counter(
		"telegram_webhook_rate_limit_exceeded_total",
		"Total de requisicoes bloqueadas pelo rate limit do webhook Telegram",
		"1",
	)

	webhookPath := cfg.TelegramConfig.WebhookPath
	if webhookPath == "" {
		webhookPath = "/api/v1/channels/telegram/webhook"
	}

	return identityserver.NewTelegramWebhookRouter(
		inboundHandler,
		cfg.TelegramConfig.SecretToken,
		cfg.TelegramConfig.SecretTokenNext,
		webhookPath,
		rateLimiter.Middleware,
		func() { rateLimitExceededTotal.Increment(context.Background()) },
	), nil
}
