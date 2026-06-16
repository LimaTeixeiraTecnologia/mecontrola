package identity

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
	identitymiddleware "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/middleware"
	jobhandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/middleware"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	tgdedup "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dedup/postgres"
	tgdispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dispatcher"
	tghandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/outbound"
	tgpayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup"
	deduppostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
)

type EventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type IdentityModule struct {
	RepositoryFactory          interfaces.RepositoryFactory
	UserRouter                 *server.UserRouter
	TelegramWebhookRouter      *server.TelegramWebhookRouter
	UpsertUserUseCase          *usecases.UpsertUserByWhatsApp
	FindUserByIDUseCase        *usecases.FindUserByID
	FindUserByWhatsApp         *usecases.FindUserByWhatsApp
	MarkUserDeleted            *usecases.MarkUserDeleted
	EstablishPrincipal         *usecases.EstablishPrincipal
	ResolvePrincipalByIdentity *usecases.ResolvePrincipalByIdentity
	LinkChannelToUser          *usecases.LinkChannelToUser
	GatewayAuthMiddleware      func(http.Handler) http.Handler
	EntitlementReader          interfaces.EntitlementReader
	SubscriptionProjector      *consumers.SubscriptionEventProjector
	SubscriptionBoundProjector *consumers.SubscriptionBoundProjector
	AuthEventsConsumer         *consumers.AuthEventsConsumer
	AuthEventsHousekeepingJob  *jobhandlers.AuthEventsHousekeepingJob
	RecordGatewayAuthFailure   *usecases.RecordGatewayAuthFailure
	ResolvePreferredChannel    *usecases.ResolvePreferredChannel
	WhatsAppLimiter            *ratelimit.Limiter
	WhatsAppDedupRepository    dedup.MessageRepository
	OutboxPublisher            outbox.Publisher
	EventHandlers              []EventHandlerRegistration
	telegramRouterBuilder      *identityModuleBuilder
}

func (m IdentityModule) BuildTelegramWebhookRouter(agentRoute tgdispatcher.AgentRoute, onboardingRoute tgdispatcher.OnboardingRoute) (*server.TelegramWebhookRouter, error) {
	if m.telegramRouterBuilder == nil {
		return nil, nil
	}
	return m.telegramRouterBuilder.buildTelegramWebhookRouter(m, agentRoute, onboardingRoute)
}

type identityModuleBuilder struct {
	cfg  *configs.Config
	o11y observability.Observability
	mgr  manager.Manager
}

type identityPublisher struct {
	mgr           manager.Manager
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	o11y          observability.Observability
}

type lazyEntitlementReader struct {
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func NewIdentityModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) (IdentityModule, error) {
	builder := &identityModuleBuilder{
		cfg:  cfg,
		o11y: o11y,
		mgr:  mgr,
	}
	return builder.build()
}

func (b *identityModuleBuilder) build() (IdentityModule, error) { //nolint:revive // wiring de módulo; cada statement é injeção de dependência sem lógica
	factory := repositories.NewRepositoryFactory(b.o11y)
	publisher := newIdentityPublisher(b.mgr, outbox.NewRepositoryFactory(b.o11y), b.cfg.OutboxConfig, b.o11y)

	upsertUoW := uow.New[entities.User](b.mgr, uow.WithObservability(b.o11y))
	markDeletedUoW := uow.NewVoid(b.mgr, uow.WithObservability(b.o11y))
	establishUoW := uow.New[usecases.EstablishResult](b.mgr, uow.WithObservability(b.o11y))
	resolveByIdentityUoW := uow.New[usecases.EstablishResult](b.mgr, uow.WithObservability(b.o11y))

	upsertUser := usecases.NewUpsertUserByWhatsApp(upsertUoW, factory, b.o11y)
	findUserByID := usecases.NewFindUserByID(b.mgr, factory, b.o11y)
	findUserByWhatsApp := usecases.NewFindUserByWhatsApp(b.mgr, factory, b.o11y)
	markUserDeleted := usecases.NewMarkUserDeleted(markDeletedUoW, factory, publisher, b.o11y)
	establishPrincipal := usecases.NewEstablishPrincipal(establishUoW, factory, publisher, b.o11y)
	resolveByIdentity := usecases.NewResolvePrincipalByIdentity(resolveByIdentityUoW, factory, b.o11y)
	linkChannelUoW := uow.New[usecases.LinkChannelResult](b.mgr, uow.WithObservability(b.o11y))
	linkChannelToUser := usecases.NewLinkChannelToUser(linkChannelUoW, factory, b.o11y)

	projectionReader := repositories.NewSubscriptionProjectionReader(b.mgr, b.o11y)
	projectSubscriptionEvent := usecases.NewProjectSubscriptionEvent(factory, b.mgr, projectionReader, b.o11y)
	subscriptionProjector := consumers.NewSubscriptionEventProjector(projectSubscriptionEvent, b.o11y)
	subscriptionBoundProjector := consumers.NewSubscriptionBoundProjector(projectSubscriptionEvent, b.o11y)

	projectAuthEvent := usecases.NewProjectAuthEvent(factory, b.mgr, b.o11y)
	anonymizeUserAuthEvents := usecases.NewAnonymizeUserAuthEvents(factory, b.mgr, b.o11y)
	authEventsConsumer := consumers.NewAuthEventsConsumer(projectAuthEvent, anonymizeUserAuthEvents, b.o11y)

	cleanupAuthEvents := usecases.NewCleanupAuthEvents(b.mgr, factory, b.cfg.IdentityConfig, b.o11y)
	authEventsHousekeepingJob := jobhandlers.NewAuthEventsHousekeepingJob(cleanupAuthEvents, b.cfg.IdentityConfig)

	resolvePreferredChannel := usecases.NewResolvePreferredChannel(b.mgr, factory, b.o11y)
	recordGatewayAuthFailure := usecases.NewRecordGatewayAuthFailure(publisher, b.o11y)
	gatewayAuthMiddleware, err := NewRequireGatewayAuth(b.cfg.IdentityConfig, recordGatewayAuthFailure, b.o11y)
	if err != nil {
		return IdentityModule{}, err
	}

	whatsAppLimiter := ratelimit.New(b.o11y)
	module := IdentityModule{
		RepositoryFactory:          factory,
		UserRouter:                 server.NewUserRouter(handlers.NewUpsertUserByWhatsAppHandler(upsertUser, b.o11y)),
		UpsertUserUseCase:          upsertUser,
		FindUserByIDUseCase:        findUserByID,
		FindUserByWhatsApp:         findUserByWhatsApp,
		MarkUserDeleted:            markUserDeleted,
		EstablishPrincipal:         establishPrincipal,
		ResolvePrincipalByIdentity: resolveByIdentity,
		LinkChannelToUser:          linkChannelToUser,
		GatewayAuthMiddleware:      gatewayAuthMiddleware,
		EntitlementReader:          &lazyEntitlementReader{mgr: b.mgr, factory: factory},
		SubscriptionProjector:      subscriptionProjector,
		SubscriptionBoundProjector: subscriptionBoundProjector,
		AuthEventsConsumer:         authEventsConsumer,
		AuthEventsHousekeepingJob:  authEventsHousekeepingJob,
		RecordGatewayAuthFailure:   recordGatewayAuthFailure,
		ResolvePreferredChannel:    resolvePreferredChannel,
		WhatsAppLimiter:            whatsAppLimiter,
		WhatsAppDedupRepository:    deduppostgres.NewMessageRepository(b.o11y, b.mgr),
		OutboxPublisher:            publisher,
		EventHandlers: []EventHandlerRegistration{
			{EventType: "billing.subscription.activated", Handler: subscriptionProjector},
			{EventType: "billing.subscription.renewed", Handler: subscriptionProjector},
			{EventType: "billing.subscription.past_due", Handler: subscriptionProjector},
			{EventType: "billing.subscription.canceled", Handler: subscriptionProjector},
			{EventType: "billing.subscription.refunded", Handler: subscriptionProjector},
			{EventType: "onboarding.subscription_bound", Handler: subscriptionBoundProjector},
			{EventType: "auth.principal_established", Handler: authEventsConsumer},
			{EventType: "auth.failed", Handler: authEventsConsumer},
			{EventType: "auth.unknown_user", Handler: authEventsConsumer},
			{EventType: "user.deleted", Handler: authEventsConsumer},
		},
	}

	module.telegramRouterBuilder = b
	return module, nil
}

func newIdentityPublisher(mgr manager.Manager, outboxFactory outbox.OutboxRepositoryFactory, cfg configs.OutboxConfig, o11y observability.Observability) outbox.Publisher {
	return &identityPublisher{mgr: mgr, outboxFactory: outboxFactory, cfg: cfg, o11y: o11y}
}

func (b *identityModuleBuilder) buildTelegramWebhookRouter(module IdentityModule, agentRouteOverride tgdispatcher.AgentRoute, onboardingRouteOverride tgdispatcher.OnboardingRoute) (*server.TelegramWebhookRouter, error) {
	if !b.cfg.TelegramConfig.Enabled {
		return nil, nil
	}

	gateway, err := outbound.NewSharedGateway(b.o11y, outbound.FactoryConfig{
		APIBaseURL: b.cfg.TelegramConfig.APIBaseURL,
		BotToken:   b.cfg.TelegramConfig.BotToken,
		Timeout:    b.cfg.TelegramConfig.OutboundTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("identity: compose telegram webhook router: %w", err)
	}
	dedupRepo := tgdedup.NewUpdateRepository(b.o11y, b.mgr)
	stubReply := b.cfg.TelegramConfig.AgentStubReceived
	onboardingReply := b.cfg.TelegramConfig.OnboardingFallback

	stubOnboardingRoute := func(ctx context.Context, msg tgpayload.Message) tgdispatcher.RouteOutcome {
		if onboardingReply == "" {
			return tgdispatcher.OutcomeFallback
		}
		if err := gateway.SendTextMessage(ctx, msg.ChatID, onboardingReply); err != nil {
			b.o11y.Logger().Warn(ctx, "telegram.dispatcher.onboarding_route_failed",
				observability.Error(err),
			)
		}
		return tgdispatcher.OutcomeFallback
	}
	onboardingRoute := stubOnboardingRoute
	if onboardingRouteOverride != nil {
		onboardingRoute = onboardingRouteOverride
	}

	stubAgentRoute := func(ctx context.Context, msg tgpayload.Message) tgdispatcher.RouteOutcome {
		if stubReply == "" {
			return tgdispatcher.OutcomeAgent
		}
		if err := gateway.SendTextMessage(ctx, msg.ChatID, stubReply); err != nil {
			b.o11y.Logger().Warn(ctx, "telegram.dispatcher.agent_route_failed",
				observability.Error(err),
			)
		}
		return tgdispatcher.OutcomeAgent
	}
	agentRoute := stubAgentRoute
	if agentRouteOverride != nil {
		agentRoute = agentRouteOverride
	}

	dispatcher := tgdispatcher.New(
		b.cfg.TelegramConfig.BotID,
		dedupRepo,
		module.ResolvePrincipalByIdentity,
		module.WhatsAppLimiter,
		module.OutboxPublisher,
		onboardingRoute,
		agentRoute,
		b.o11y,
	)

	inboundHandler := tghandlers.NewInboundHandler(dispatcher, b.o11y)
	rateLimiter := middleware.NewRateLimiter(
		b.cfg.TelegramConfig.WebhookRateLimitPerMin,
		b.cfg.TelegramConfig.WebhookRateLimitBurst,
		b.parseCSV(b.cfg.OnboardingConfig.TrustedProxies),
	)
	rateLimitExceededTotal := b.o11y.Metrics().Counter(
		"telegram_webhook_rate_limit_exceeded_total",
		"Total de requisicoes bloqueadas pelo rate limit do webhook Telegram",
		"1",
	)

	webhookPath := b.cfg.TelegramConfig.WebhookPath
	if webhookPath == "" {
		webhookPath = "/api/v1/channels/telegram/webhook"
	}

	return server.NewTelegramWebhookRouter(
		inboundHandler,
		b.cfg.TelegramConfig.SecretToken,
		b.cfg.TelegramConfig.SecretTokenNext,
		webhookPath,
		rateLimiter.Middleware,
		func() { rateLimitExceededTotal.Increment(context.Background()) },
	), nil
}

func (b *identityModuleBuilder) parseCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	values := make([]string, 0)
	for item := range strings.SplitSeq(raw, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return values
}

func NewRequireGatewayAuth(cfg configs.IdentityConfig, failureUseCase *usecases.RecordGatewayAuthFailure, o11y observability.Observability) (func(http.Handler) http.Handler, error) {
	current, err := decodeGatewaySecret("current", cfg.GatewaySharedSecretCurrent)
	if err != nil {
		return nil, err
	}
	next, err := decodeGatewaySecret("next", cfg.GatewaySharedSecretNext)
	if err != nil {
		return nil, err
	}
	window := cfg.GatewayAuthWindow
	if window <= 0 {
		window = 60 * time.Second
	}
	deps := identitymiddleware.RequireGatewayAuthDeps{
		Secrets:       services.SecretPair{Current: current, Next: next},
		Window:        window,
		FailureLogger: failureUseCase,
		O11y:          o11y,
	}
	return identitymiddleware.RequireGatewayAuth(deps), nil
}

func decodeGatewaySecret(label string, raw string) ([]byte, error) {
	if raw == "" {
		return nil, nil
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("identity: decode gateway secret %s: %w", label, err)
	}
	return decoded, nil
}

func (r *lazyEntitlementReader) FindByUserID(ctx context.Context, userID string) (interfaces.EntitlementRecord, error) {
	return r.factory.EntitlementRepository(r.mgr.DBTX(ctx)).FindByUserID(ctx, userID)
}

func (p *identityPublisher) Publish(ctx context.Context, evt outbox.Event) error {
	storage := p.outboxFactory.OutboxRepository(p.mgr.DBTX(ctx))
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)
	return publisher.Publish(ctx, evt)
}
