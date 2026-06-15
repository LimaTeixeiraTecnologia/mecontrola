package onboarding

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	identityinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/binding"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/checkout"
	onboardingconfig "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/config"
	onboardingcrypto "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/crypto"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/gateway"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/client/meta"
	onboardingserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/middleware"
	onboardingjobs "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
)

type EventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type OnboardingModule struct {
	PublicRouter                 *onboardingserver.PublicRouter
	WhatsAppGateway              appinterfaces.WhatsAppGateway
	WhatsAppMessageProcessor     *services.WhatsAppMessageProcessor
	TelegramMessageProcessor     *services.TelegramMessageProcessor
	SubscriptionConsumer         events.Handler
	PaidWithoutTokenConsumer     events.Handler
	OutreachJob                  worker.Job
	ExpirationJob                worker.Job
	MetaProcessedMessagesCleanup worker.Job
	EventHandlers                []EventHandlerRegistration
}

type managerPublisher struct {
	mgr           manager.Manager
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	o11y          observability.Observability
}

type identityGatewayAdapter struct {
	identityModule identity.IdentityModule
}

func buildTelegramMessages(tgCfg configs.TelegramConfig) map[string]string {
	return map[string]string{
		"welcome_activated":               tgCfg.WelcomeActivated,
		"already_active":                  tgCfg.AlreadyActive,
		"requires_whatsapp_activation":    tgCfg.RequiresWhatsApp,
		"code_already_used_other_account": tgCfg.CodeAlreadyUsed,
		"payment_still_processing_retry":  tgCfg.PaymentProcessing,
		"code_expired_contact_support":    tgCfg.CodeExpired,
		"code_invalid_check_again":        tgCfg.CodeInvalid,
		"system_unavailable_retry":        tgCfg.SystemUnavailable,
		"please_use_ativar_command":       tgCfg.PleaseUseAtivar,
	}
}

func NewOnboardingModule(
	mgr manager.Manager,
	cfg configs.OnboardingConfig,
	waCfg configs.WhatsAppConfig,
	tgCfg configs.TelegramConfig,
	outboxCfg configs.OutboxConfig,
	identityModule identity.IdentityModule,
	o11y observability.Observability,
) (OnboardingModule, error) {
	runtimeCfg, err := onboardingconfig.NewOnboardingRuntimeConfig(cfg, waCfg)
	if err != nil {
		return OnboardingModule{}, err
	}
	factory := repositories.NewRepositoryFactory(o11y)
	publisher := newManagerPublisher(mgr, outbox.NewRepositoryFactory(o11y), outboxCfg, o11y)
	idGen := id.NewUUIDGenerator()

	tokenCipher, err := onboardingcrypto.NewTokenCipher(cfg.TokenEncryptionKey)
	if err != nil {
		return OnboardingModule{}, err
	}
	whatsAppGateway, err := newWhatsAppGateway(waCfg, o11y)
	if err != nil {
		return OnboardingModule{}, err
	}

	identityGateway := newIdentityGatewayAdapter(identityModule)
	subscriptionBinder := postgres.NewSubscriptionBinder(o11y, mgr.DBTX(context.Background()))
	urlBuilder := checkout.NewKiwifyURLBuilder(runtimeCfg.CheckoutURLs, runtimeCfg.KiwifyAllowedHosts)
	checkoutUoW := uow.New[entities.MagicToken](mgr, uow.WithObservability(o11y))
	consumeUoW := uow.New[usecases.ConsumeInternalResult](mgr, uow.WithObservability(o11y))
	workflow := domainservices.NewMagicTokenWorkflow()
	bindingService := binding.NewSubscriptionBindingService(identityGateway, subscriptionBinder, workflow, publisher, idGen)

	createCheckout := usecases.NewCreateCheckoutSession(checkoutUoW, factory, urlBuilder, tokenCipher, idGen, runtimeCfg.TokenTTL, o11y)
	markTokenPaid := usecases.NewMarkTokenPaid(mgr, factory, workflow, o11y)
	consumeToken := usecases.NewConsumeMagicToken(consumeUoW, factory, bindingService, idGen, o11y)
	fallbackActivation := usecases.NewTryFallbackActivation(consumeUoW, factory, bindingService, o11y)
	getTokenState := usecases.NewGetTokenState(mgr, factory, waCfg.BotNumberE164, waCfg.BotNumberDisplay, o11y)
	sendOutreach := usecases.NewSendOutreach(mgr, factory, whatsAppGateway, tokenCipher, idGen, waCfg.OutreachTemplateName, runtimeCfg.OutreachGap, o11y)
	expireTokens := usecases.NewExpireTokens(mgr, factory, idGen, o11y)
	handlePaidWithoutToken := usecases.NewHandlePaidWithoutToken(mgr, factory, idGen, o11y)
	cleanupTables := usecases.NewCleanupOnboardingTables(mgr.DBTX(context.Background()), factory, runtimeCfg.MetaRetention, o11y)

	registerMetrics(mgr, factory, o11y)
	subscriptionConsumer := consumers.NewSubscriptionPaidConsumer(markTokenPaid, o11y)
	paidWithoutTokenConsumer := consumers.NewPaidWithoutTokenConsumer(handlePaidWithoutToken, o11y)
	publicRouter := newPublicRouter(cfg, runtimeCfg, createCheckout, getTokenState, o11y)
	messageProcessor := services.NewWhatsAppMessageProcessor(
		consumeToken,
		fallbackActivation,
		whatsAppGateway,
		runtimeCfg.Messages,
		o11y,
	)
	activateTelegramUoW := uow.New[usecases.ActivateTelegramResult](mgr, uow.WithObservability(o11y))
	activateTelegram := usecases.NewActivateTelegramByToken(factory, identityModule.RepositoryFactory, activateTelegramUoW, o11y)
	telegramProcessor := services.NewTelegramMessageProcessor(
		activateTelegram,
		buildTelegramMessages(tgCfg),
		o11y,
	)

	return OnboardingModule{
		PublicRouter:                 publicRouter,
		WhatsAppGateway:              whatsAppGateway,
		WhatsAppMessageProcessor:     messageProcessor,
		TelegramMessageProcessor:     telegramProcessor,
		SubscriptionConsumer:         subscriptionConsumer,
		PaidWithoutTokenConsumer:     paidWithoutTokenConsumer,
		OutreachJob:                  onboardingjobs.NewOutreachJob(sendOutreach, cfg.OutreachEnabled),
		ExpirationJob:                onboardingjobs.NewTokenExpirationJob(expireTokens, cfg.TokenExpirationSchedule),
		MetaProcessedMessagesCleanup: onboardingjobs.NewMetaProcessedMessagesCleanupJob(cleanupTables, cfg.MetaCleanupSchedule),
		EventHandlers: []EventHandlerRegistration{
			{EventType: "billing.subscription.activated", Handler: subscriptionConsumer},
			{EventType: "billing.subscription.activated_without_token", Handler: paidWithoutTokenConsumer},
		},
	}, nil
}

func newManagerPublisher(
	mgr manager.Manager,
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	o11y observability.Observability,
) outbox.Publisher {
	return &managerPublisher{mgr: mgr, outboxFactory: outboxFactory, cfg: cfg, o11y: o11y}
}

func newWhatsAppGateway(waCfg configs.WhatsAppConfig, o11y observability.Observability) (appinterfaces.WhatsAppGateway, error) {
	metaClient, err := meta.NewClient(o11y, meta.Config{
		PhoneNumberID: waCfg.PhoneNumberID,
		AccessToken:   waCfg.AccessToken,
	})
	if err != nil {
		return nil, fmt.Errorf("onboarding: criar cliente meta: %w", err)
	}
	return gateway.NewWhatsAppGateway(metaClient), nil
}

func newIdentityGatewayAdapter(identityModule identity.IdentityModule) appinterfaces.IdentityGateway {
	return &identityGatewayAdapter{identityModule: identityModule}
}

func registerMetrics(
	mgr manager.Manager,
	factory appinterfaces.RepositoryFactory,
	o11y observability.Observability,
) {
	_ = o11y.Metrics().Gauge(
		"onboarding_tokens_paid_unconsumed",
		"Total de tokens no estado PAID ainda nao consumidos",
		"1",
		func(ctx context.Context) float64 {
			repo := factory.MagicTokenRepository(mgr.DBTX(ctx))
			count, err := repo.CountPaidUnconsumed(ctx)
			if err != nil {
				return -1
			}
			return float64(count)
		},
	)
}

func newPublicRouter(
	cfg configs.OnboardingConfig,
	runtimeCfg onboardingconfig.OnboardingRuntimeConfig,
	createCheckout *usecases.CreateCheckoutSession,
	getTokenState *usecases.GetTokenState,
	o11y observability.Observability,
) *onboardingserver.PublicRouter {
	trustedProxies := runtimeCfg.TrustedProxies
	checkoutCreatedCounter := o11y.Metrics().Counter(
		"onboarding_checkout_sessions_created_total",
		"Total de sessoes de checkout criadas",
		"1",
	)
	checkoutRateLimitedCounter := o11y.Metrics().Counter(
		"onboarding_checkout_rate_limited_total",
		"Total de requisicoes de checkout bloqueadas por rate limit",
		"1",
	)
	invalidAccessCounter := o11y.Metrics().Counter(
		"ty_page_invalid_access_total",
		"Total de acessos invalidos a pagina de obrigado",
		"1",
	)

	checkoutLimiter := middleware.NewRateLimiter(
		cfg.CheckoutRateLimitPerMin,
		cfg.CheckoutRateLimitBurst,
		trustedProxies,
	)
	stateLimiter := middleware.NewRateLimiter(
		cfg.StateRateLimitPerMin,
		cfg.StateRateLimitBurst,
		trustedProxies,
	)

	checkoutHandler := handlers.NewCreateCheckoutHandler(
		createCheckout,
		func(planID string) {
			checkoutCreatedCounter.Add(context.Background(), 1, observability.String("plan_id", planID))
		},
		func() {
			checkoutRateLimitedCounter.Add(context.Background(), 1)
		},
		o11y,
	)
	stateHandler := handlers.NewTokenStateHandler(
		getTokenState,
		func(reason string) {
			invalidAccessCounter.Add(context.Background(), 1, observability.String("reason", reason))
		},
		o11y,
	)

	return onboardingserver.NewPublicRouter(
		checkoutHandler,
		stateHandler,
		checkoutLimiter,
		stateLimiter,
		runtimeCfg.CheckoutCORSOrigins,
	)
}

func (p *managerPublisher) Publish(ctx context.Context, evt outbox.Event) error {
	storage := p.outboxFactory.OutboxRepository(p.mgr.DBTX(ctx))
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)
	return publisher.Publish(ctx, evt)
}

func (a *identityGatewayAdapter) UpsertUserByWhatsApp(
	ctx context.Context,
	mobileE164 string,
	email string,
) (appinterfaces.UpsertUserResult, error) {
	result, err := a.identityModule.UpsertUserUseCase.Execute(ctx, identityinput.UpsertUserByWhatsApp{
		WhatsAppNumber: mobileE164,
		Email:          email,
	})
	if err != nil {
		return appinterfaces.UpsertUserResult{}, fmt.Errorf("onboarding: identity gateway: upsert user: %w", err)
	}
	return appinterfaces.UpsertUserResult{UserID: result.ID}, nil
}
