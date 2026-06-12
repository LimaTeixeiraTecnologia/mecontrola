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
	SubscriptionConsumer         events.Handler
	PaidWithoutTokenConsumer     events.Handler
	OutreachJob                  worker.Job
	ExpirationJob                worker.Job
	MetaProcessedMessagesCleanup worker.Job
	EventHandlers                []EventHandlerRegistration
}

type moduleBuilder struct {
	mgr            manager.Manager
	cfg            configs.OnboardingConfig
	waCfg          configs.WhatsAppConfig
	outboxCfg      configs.OutboxConfig
	runtimeCfg     onboardingconfig.OnboardingRuntimeConfig
	identityModule identity.IdentityModule
	o11y           observability.Observability
	factory        appinterfaces.RepositoryFactory
	publisher      outbox.Publisher
	idGen          id.Generator
}

type moduleRuntime struct {
	createCheckout         *usecases.CreateCheckoutSession
	markTokenPaid          *usecases.MarkTokenPaid
	consumeToken           *usecases.ConsumeMagicToken
	fallbackActivation     *usecases.TryFallbackActivation
	getTokenState          *usecases.GetTokenState
	sendOutreach           *usecases.SendOutreach
	expireTokens           *usecases.ExpireTokens
	handlePaidWithoutToken *usecases.HandlePaidWithoutToken
	cleanupTables          *usecases.CleanupOnboardingTables
	whatsAppGateway        appinterfaces.WhatsAppGateway
}

type managerPublisher struct {
	mgr           manager.Manager
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
}

type identityGatewayAdapter struct {
	identityModule identity.IdentityModule
}

func NewOnboardingModule(
	mgr manager.Manager,
	cfg configs.OnboardingConfig,
	waCfg configs.WhatsAppConfig,
	outboxCfg configs.OutboxConfig,
	identityModule identity.IdentityModule,
	o11y observability.Observability,
) (OnboardingModule, error) {
	builder, err := newModuleBuilder(mgr, cfg, waCfg, outboxCfg, identityModule, o11y)
	if err != nil {
		return OnboardingModule{}, err
	}
	return builder.Build()
}

func newModuleBuilder(
	mgr manager.Manager,
	cfg configs.OnboardingConfig,
	waCfg configs.WhatsAppConfig,
	outboxCfg configs.OutboxConfig,
	identityModule identity.IdentityModule,
	o11y observability.Observability,
) (*moduleBuilder, error) {
	runtimeCfg, err := onboardingconfig.NewOnboardingRuntimeConfig(cfg, waCfg)
	if err != nil {
		return nil, err
	}
	factory := repositories.NewRepositoryFactory(o11y)
	return &moduleBuilder{
		mgr:            mgr,
		cfg:            cfg,
		waCfg:          waCfg,
		outboxCfg:      outboxCfg,
		runtimeCfg:     runtimeCfg,
		identityModule: identityModule,
		o11y:           o11y,
		factory:        factory,
		publisher:      newManagerPublisher(mgr, outbox.NewRepositoryFactory(o11y), outboxCfg),
		idGen:          id.NewUUIDGenerator(),
	}, nil
}

func newManagerPublisher(
	mgr manager.Manager,
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
) outbox.Publisher {
	return &managerPublisher{mgr: mgr, outboxFactory: outboxFactory, cfg: cfg}
}

func newIdentityGatewayAdapter(identityModule identity.IdentityModule) appinterfaces.IdentityGateway {
	return &identityGatewayAdapter{identityModule: identityModule}
}

func (b *moduleBuilder) Build() (OnboardingModule, error) {
	runtime, err := b.buildRuntime()
	if err != nil {
		return OnboardingModule{}, err
	}

	b.registerMetrics()

	subscriptionConsumer := consumers.NewSubscriptionPaidConsumer(runtime.markTokenPaid, b.o11y)
	paidWithoutTokenConsumer := consumers.NewPaidWithoutTokenConsumer(runtime.handlePaidWithoutToken, b.o11y)
	publicRouter := b.buildPublicRouter(runtime)
	messageProcessor := b.buildMessageProcessor(runtime)

	return OnboardingModule{
		PublicRouter:                 publicRouter,
		WhatsAppGateway:              runtime.whatsAppGateway,
		WhatsAppMessageProcessor:     messageProcessor,
		SubscriptionConsumer:         subscriptionConsumer,
		PaidWithoutTokenConsumer:     paidWithoutTokenConsumer,
		OutreachJob:                  onboardingjobs.NewOutreachJob(runtime.sendOutreach, b.cfg.OutreachEnabled),
		ExpirationJob:                onboardingjobs.NewTokenExpirationJob(runtime.expireTokens, b.cfg.TokenExpirationSchedule),
		MetaProcessedMessagesCleanup: onboardingjobs.NewMetaProcessedMessagesCleanupJob(runtime.cleanupTables, b.cfg.MetaCleanupSchedule),
		EventHandlers:                b.buildEventHandlers(subscriptionConsumer, paidWithoutTokenConsumer),
	}, nil
}

func (b *moduleBuilder) buildRuntime() (moduleRuntime, error) {
	tokenCipher, err := b.buildTokenCipher()
	if err != nil {
		return moduleRuntime{}, err
	}

	whatsAppGateway, err := b.buildWhatsAppGateway()
	if err != nil {
		return moduleRuntime{}, err
	}

	identityGateway := b.buildIdentityGateway()
	subscriptionBinder := b.buildSubscriptionBinder()

	return b.buildUseCases(tokenCipher, whatsAppGateway, identityGateway, subscriptionBinder), nil
}

func (b *moduleBuilder) buildTokenCipher() (appinterfaces.TokenCipher, error) {
	tokenCipher, err := onboardingcrypto.NewTokenCipher(b.cfg.TokenEncryptionKey)
	if err != nil {
		return nil, err
	}
	return tokenCipher, nil
}

func (b *moduleBuilder) buildWhatsAppGateway() (appinterfaces.WhatsAppGateway, error) {
	metaClient, err := meta.NewClient(b.o11y, meta.Config{
		PhoneNumberID: b.waCfg.PhoneNumberID,
		AccessToken:   b.waCfg.AccessToken,
	})
	if err != nil {
		return nil, fmt.Errorf("onboarding: criar cliente meta: %w", err)
	}
	return gateway.NewWhatsAppGateway(metaClient), nil
}

func (b *moduleBuilder) buildIdentityGateway() appinterfaces.IdentityGateway {
	return newIdentityGatewayAdapter(b.identityModule)
}

func (b *moduleBuilder) buildSubscriptionBinder() appinterfaces.SubscriptionBinder {
	return postgres.NewSubscriptionBinder(b.o11y, b.mgr.DBTX(context.Background()))
}

func (b *moduleBuilder) buildUseCases(
	tokenCipher appinterfaces.TokenCipher,
	whatsAppGateway appinterfaces.WhatsAppGateway,
	identityGateway appinterfaces.IdentityGateway,
	subscriptionBinder appinterfaces.SubscriptionBinder,
) moduleRuntime {
	urlBuilder := checkout.NewKiwifyURLBuilder(b.runtimeCfg.CheckoutURLs, b.runtimeCfg.KiwifyAllowedHosts)
	checkoutUoW := uow.New[entities.MagicToken](b.mgr, uow.WithObservability(b.o11y))
	consumeUoW := uow.New[usecases.ConsumeInternalResult](b.mgr, uow.WithObservability(b.o11y))
	workflow := domainservices.NewMagicTokenWorkflow()
	bindingService := binding.NewSubscriptionBindingService(identityGateway, subscriptionBinder, workflow, b.publisher, b.idGen)

	return moduleRuntime{
		createCheckout:         usecases.NewCreateCheckoutSession(checkoutUoW, b.factory, urlBuilder, tokenCipher, b.idGen, b.runtimeCfg.TokenTTL, b.o11y),
		markTokenPaid:          usecases.NewMarkTokenPaid(b.mgr, b.factory, workflow, b.o11y),
		consumeToken:           usecases.NewConsumeMagicToken(consumeUoW, b.factory, bindingService, b.idGen, b.o11y),
		fallbackActivation:     usecases.NewTryFallbackActivation(consumeUoW, b.factory, bindingService, b.o11y),
		getTokenState:          usecases.NewGetTokenState(b.mgr, b.factory, b.waCfg.BotNumberE164, b.waCfg.BotNumberDisplay, b.o11y),
		sendOutreach:           usecases.NewSendOutreach(b.mgr, b.factory, whatsAppGateway, tokenCipher, b.idGen, b.waCfg.OutreachTemplateName, b.runtimeCfg.OutreachGap, b.o11y),
		expireTokens:           usecases.NewExpireTokens(b.mgr, b.factory, b.idGen, b.o11y),
		handlePaidWithoutToken: usecases.NewHandlePaidWithoutToken(b.mgr, b.factory, b.idGen, b.o11y),
		cleanupTables:          usecases.NewCleanupOnboardingTables(b.mgr.DBTX(context.Background()), b.factory, b.runtimeCfg.MetaRetention, b.o11y),
		whatsAppGateway:        whatsAppGateway,
	}
}

func (b *moduleBuilder) registerMetrics() {
	_ = b.o11y.Metrics().Gauge(
		"onboarding_tokens_paid_unconsumed",
		"Total de tokens no estado PAID ainda nao consumidos",
		"1",
		func(ctx context.Context) float64 {
			repo := b.factory.MagicTokenRepository(b.mgr.DBTX(ctx))
			count, err := repo.CountPaidUnconsumed(ctx)
			if err != nil {
				return -1
			}
			return float64(count)
		},
	)
}

func (b *moduleBuilder) buildPublicRouter(runtime moduleRuntime) *onboardingserver.PublicRouter {
	trustedProxies := b.runtimeCfg.TrustedProxies
	checkoutCreatedCounter := b.o11y.Metrics().Counter(
		"onboarding_checkout_sessions_created_total",
		"Total de sessoes de checkout criadas",
		"1",
	)
	checkoutRateLimitedCounter := b.o11y.Metrics().Counter(
		"onboarding_checkout_rate_limited_total",
		"Total de requisicoes de checkout bloqueadas por rate limit",
		"1",
	)
	invalidAccessCounter := b.o11y.Metrics().Counter(
		"ty_page_invalid_access_total",
		"Total de acessos invalidos a pagina de obrigado",
		"1",
	)

	checkoutLimiter := middleware.NewRateLimiter(
		b.cfg.CheckoutRateLimitPerMin,
		b.cfg.CheckoutRateLimitBurst,
		trustedProxies,
	)
	stateLimiter := middleware.NewRateLimiter(
		b.cfg.StateRateLimitPerMin,
		b.cfg.StateRateLimitBurst,
		trustedProxies,
	)

	checkoutHandler := handlers.NewCreateCheckoutHandler(
		runtime.createCheckout,
		func(planID string) {
			checkoutCreatedCounter.Add(context.Background(), 1, observability.String("plan_id", planID))
		},
		func() {
			checkoutRateLimitedCounter.Add(context.Background(), 1)
		},
		b.o11y,
	)
	stateHandler := handlers.NewTokenStateHandler(
		runtime.getTokenState,
		func(reason string) {
			invalidAccessCounter.Add(context.Background(), 1, observability.String("reason", reason))
		},
		b.o11y,
	)

	return onboardingserver.NewPublicRouter(
		checkoutHandler,
		stateHandler,
		checkoutLimiter,
		stateLimiter,
		b.runtimeCfg.CheckoutCORSOrigins,
	)
}

func (b *moduleBuilder) buildMessageProcessor(runtime moduleRuntime) *services.WhatsAppMessageProcessor {
	return services.NewWhatsAppMessageProcessor(
		runtime.consumeToken,
		runtime.fallbackActivation,
		runtime.whatsAppGateway,
		b.runtimeCfg.Messages,
		b.o11y,
	)
}

func (b *moduleBuilder) buildEventHandlers(
	subscriptionConsumer events.Handler,
	paidWithoutTokenConsumer events.Handler,
) []EventHandlerRegistration {
	return []EventHandlerRegistration{
		{EventType: "billing.subscription.activated", Handler: subscriptionConsumer},
		{EventType: "billing.subscription.activated_without_token", Handler: paidWithoutTokenConsumer},
	}
}

func (p *managerPublisher) Publish(ctx context.Context, evt outbox.Event) error {
	storage := p.outboxFactory.OutboxRepository(p.mgr.DBTX(ctx))
	publisher := outbox.NewPostgresPublisher(storage, p.cfg)
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
