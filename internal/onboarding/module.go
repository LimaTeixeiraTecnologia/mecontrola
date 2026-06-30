package onboarding

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	identityinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/binding"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/checkout"
	onboardingconfig "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/config"
	onboardingcrypto "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/crypto"
	onboardingemail "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/email"
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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification/adapters"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wapayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
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
	WhatsAppActivationRoute      func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome
	SubscriptionConsumer         events.Handler
	PaidWithoutTokenConsumer     events.Handler
	OutreachJob                  worker.Job
	ExpirationJob                worker.Job
	MetaProcessedMessagesCleanup worker.Job
	SendActivationEmail          *usecases.SendActivationEmail
	EventHandlers                []EventHandlerRegistration
}

type managerPublisher struct {
	db            database.DBTX
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	o11y          observability.Observability
}

type identityGatewayAdapter struct {
	identityModule identity.IdentityModule
}

type onboardingDependencies struct {
	runtimeCfg      onboardingconfig.OnboardingRuntimeConfig
	factory         appinterfaces.RepositoryFactory
	publisher       outbox.Publisher
	idGen           id.Generator
	whatsAppGateway appinterfaces.WhatsAppGateway
	bindingService  *binding.SubscriptionBindingService
}

type onboardingUseCasesBundle struct {
	createCheckout         *usecases.CreateCheckoutSession
	markTokenPaid          *usecases.MarkTokenPaid
	consumeToken           *usecases.ConsumeMagicToken
	fallbackActivation     *usecases.TryFallbackActivation
	activateFromInbound    *usecases.ActivateFromInbound
	getTokenState          *usecases.GetTokenState
	sendOutreach           *usecases.SendOutreach
	sendActivationEmail    *usecases.SendActivationEmail
	expireTokens           *usecases.ExpireTokens
	handlePaidWithoutToken *usecases.HandlePaidWithoutToken
	cleanupTables          *usecases.CleanupOnboardingTables
	recordJourneyTimestamp *usecases.RecordJourneyTimestamp
}

func NewOnboardingModule(
	db *sqlx.DB,
	cfg configs.OnboardingConfig,
	waCfg configs.WhatsAppConfig,
	outboxCfg configs.OutboxConfig,
	emailCfg configs.EmailConfig,
	identityModule identity.IdentityModule,
	o11y observability.Observability,
) (OnboardingModule, error) {
	deps, err := buildOnboardingDependencies(db, cfg, waCfg, outboxCfg, identityModule, o11y)
	if err != nil {
		return OnboardingModule{}, err
	}
	useCases, err := buildOnboardingUseCases(db, cfg, waCfg, emailCfg, deps, o11y)
	if err != nil {
		return OnboardingModule{}, err
	}
	registerMetrics(db, deps.factory, o11y)
	subscriptionConsumer := consumers.NewSubscriptionPaidConsumer(useCases.markTokenPaid, o11y)
	paidWithoutTokenConsumer := consumers.NewPaidWithoutTokenConsumer(useCases.handlePaidWithoutToken, o11y)
	activationEmailConsumer := consumers.NewActivationEmailConsumer(useCases.sendActivationEmail, o11y)
	activationAttemptConsumer := consumers.NewActivationAttemptConsumer(useCases.activateFromInbound, o11y)
	welcomeConsumer := consumers.NewWelcomeConsumer(
		deps.whatsAppGateway,
		postgres.NewWelcomeDedupRepository(o11y, db),
		deps.runtimeCfg.Messages["welcome_activated"],
		deps.runtimeCfg.Messages["onboarding_intro"],
		time.Duration(cfg.ActivationWindowHours)*time.Hour,
		o11y,
	)

	return OnboardingModule{
		PublicRouter:                 newPublicRouter(cfg, deps.runtimeCfg, useCases.createCheckout, useCases.getTokenState, useCases.recordJourneyTimestamp, o11y),
		WhatsAppGateway:              deps.whatsAppGateway,
		WhatsAppMessageProcessor:     newWhatsAppMessageProcessor(useCases, deps, o11y),
		WhatsAppActivationRoute:      buildWhatsAppActivationRoute(deps.publisher, o11y),
		SubscriptionConsumer:         subscriptionConsumer,
		PaidWithoutTokenConsumer:     paidWithoutTokenConsumer,
		OutreachJob:                  onboardingjobs.NewOutreachJob(useCases.sendOutreach, cfg.OutreachEnabled),
		ExpirationJob:                onboardingjobs.NewTokenExpirationJob(useCases.expireTokens, cfg.TokenExpirationSchedule),
		MetaProcessedMessagesCleanup: onboardingjobs.NewMetaProcessedMessagesCleanupJob(useCases.cleanupTables, cfg.MetaCleanupSchedule),
		SendActivationEmail:          useCases.sendActivationEmail,
		EventHandlers: []EventHandlerRegistration{
			{EventType: "billing.subscription.activated", Handler: subscriptionConsumer},
			{EventType: "billing.subscription.activated", Handler: activationEmailConsumer},
			{EventType: "billing.subscription.activated_without_token", Handler: paidWithoutTokenConsumer},
			{EventType: "onboarding.activation.attempted.v1", Handler: activationAttemptConsumer},
			{EventType: "onboarding.subscription_bound", Handler: welcomeConsumer},
		},
	}, nil
}

type activationAttemptedPayload struct {
	PeerE164  string `json:"peer_e164"`
	Text      string `json:"text"`
	MessageID string `json:"message_id"`
}

func buildWhatsAppActivationRoute(
	publisher outbox.Publisher,
	o11y observability.Observability,
) func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
	routeTotal := o11y.Metrics().Counter(
		"onboarding_activation_route_total",
		"Total de tentativas de ativacao roteadas pelo dispatcher de WhatsApp",
		"1",
	)
	return func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
		ctx, span := o11y.Tracer().Start(ctx, "onboarding.route.whatsapp_activation")
		defer span.End()

		p := activationAttemptedPayload{
			PeerE164:  msg.From,
			Text:      msg.Text,
			MessageID: msg.WAMID,
		}
		raw, err := json.Marshal(p)
		if err != nil {
			o11y.Logger().Error(ctx, "onboarding.route.whatsapp_activation: marshal payload", observability.Error(err))
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		eventID, err := uuid.NewV7()
		if err != nil {
			o11y.Logger().Error(ctx, "onboarding.route.whatsapp_activation: gerar event id", observability.Error(err))
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		evt, err := outbox.NewEvent(outbox.EventInput{
			ID:            eventID.String(),
			Type:          "onboarding.activation.attempted.v1",
			AggregateType: "whatsapp.message",
			AggregateID:   msg.WAMID,
			Payload:       raw,
			OccurredAt:    time.Now().UTC(),
		})
		if err != nil {
			o11y.Logger().Error(ctx, "onboarding.route.whatsapp_activation: criar evento", observability.Error(err))
			span.RecordError(err)
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		if err := publisher.Publish(ctx, evt); err != nil {
			o11y.Logger().Error(ctx, "onboarding.route.whatsapp_activation: publicar evento", observability.Error(err))
			span.RecordError(err)
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		routeTotal.Add(ctx, 1, observability.String("outcome", "routed"))
		return wadispatcher.OutcomeNoRoute
	}
}

func buildOnboardingDependencies(
	db *sqlx.DB,
	cfg configs.OnboardingConfig,
	waCfg configs.WhatsAppConfig,
	outboxCfg configs.OutboxConfig,
	identityModule identity.IdentityModule,
	o11y observability.Observability,
) (onboardingDependencies, error) {
	runtimeCfg, err := onboardingconfig.NewOnboardingRuntimeConfig(cfg, waCfg)
	if err != nil {
		return onboardingDependencies{}, err
	}
	whatsAppGateway, err := newWhatsAppGateway(waCfg, o11y)
	if err != nil {
		return onboardingDependencies{}, err
	}
	factory := repositories.NewRepositoryFactory(o11y)
	publisher := newManagerPublisher(db, outbox.NewRepositoryFactory(o11y), outboxCfg, o11y)
	idGen := id.NewUUIDGenerator()
	identityGateway := newIdentityGatewayAdapter(identityModule)
	subscriptionBinder := postgres.NewSubscriptionBinder(o11y, db)
	workflow := domainservices.NewMagicTokenWorkflow()
	return onboardingDependencies{
		runtimeCfg:      runtimeCfg,
		factory:         factory,
		publisher:       publisher,
		idGen:           idGen,
		whatsAppGateway: whatsAppGateway,
		bindingService:  binding.NewSubscriptionBindingService(identityGateway, subscriptionBinder, workflow, publisher, idGen),
	}, nil
}

func buildOnboardingUseCases(
	db *sqlx.DB,
	cfg configs.OnboardingConfig,
	waCfg configs.WhatsAppConfig,
	emailCfg configs.EmailConfig,
	deps onboardingDependencies,
	o11y observability.Observability,
) (onboardingUseCasesBundle, error) {
	tokenCipher, err := onboardingcrypto.NewTokenCipher(cfg.TokenEncryptionKey)
	if err != nil {
		return onboardingUseCasesBundle{}, err
	}
	channelGateway := buildNotificationChannelGateway(deps.whatsAppGateway)
	emailSender, err := onboardingemail.NewSenderFactory(emailCfg, o11y).Build()
	if err != nil {
		return onboardingUseCasesBundle{}, fmt.Errorf("onboarding: build email sender: %w", err)
	}
	checkoutUoW := uow.NewUnitOfWork(db)
	consumeUoW := uow.NewUnitOfWork(db)
	activateUoW := uow.NewUnitOfWork(db)
	urlBuilder := checkout.NewKiwifyURLBuilder(deps.runtimeCfg.CheckoutURLs, deps.runtimeCfg.KiwifyAllowedHosts)
	activationTemplate := onboardingemail.NewActivationTemplate()
	magicTokenWorkflow := domainservices.NewMagicTokenWorkflow()
	magicTokenRepo := deps.factory.MagicTokenRepository(db)
	supportSignalRepo := deps.factory.SupportSignalRepository(db)
	cleanupRepo := deps.factory.OnboardingCleanupRepository(db)
	activationWindow := time.Duration(cfg.ActivationWindowHours) * time.Hour
	consumeToken := usecases.NewConsumeMagicToken(consumeUoW, deps.factory, deps.bindingService, deps.idGen, activationWindow, o11y)
	noMatchThrottle := postgres.NewNoMatchThrottleRepository(o11y, db)
	return onboardingUseCasesBundle{
		createCheckout:     usecases.NewCreateCheckoutSession(checkoutUoW, deps.factory, urlBuilder, tokenCipher, deps.idGen, deps.runtimeCfg.TokenTTL, o11y),
		markTokenPaid:      usecases.NewMarkTokenPaid(magicTokenRepo, magicTokenWorkflow, o11y),
		consumeToken:       consumeToken,
		fallbackActivation: usecases.NewTryFallbackActivation(consumeUoW, deps.factory, deps.bindingService, o11y),
		activateFromInbound: usecases.NewActivateFromInbound(
			activateUoW,
			deps.factory,
			deps.bindingService,
			consumeToken,
			deps.whatsAppGateway,
			noMatchThrottle,
			activationWindow,
			waCfg.ActivationNotFound,
			o11y,
		),
		getTokenState: usecases.NewGetTokenState(magicTokenRepo, waCfg.BotNumberE164, waCfg.BotNumberDisplay, o11y),
		sendOutreach:  usecases.NewSendOutreach(magicTokenRepo, channelGateway, tokenCipher, deps.idGen, waCfg.OutreachTemplateName, deps.runtimeCfg.OutreachGap, o11y),
		sendActivationEmail: usecases.NewSendActivationEmail(
			magicTokenRepo,
			emailSender,
			activationTemplate,
			waCfg.BotNumberE164,
			cfg.ActivationPageURL,
			emailCfg.FromAddress,
			emailCfg.FromName,
			emailCfg.ReplyTo,
			deps.runtimeCfg.TokenTTL,
			postgres.NewSubscriptionBoundChecker(o11y, db),
			o11y,
		),
		expireTokens:           usecases.NewExpireTokens(db, deps.factory, deps.idGen, o11y),
		handlePaidWithoutToken: usecases.NewHandlePaidWithoutToken(supportSignalRepo, deps.idGen, o11y),
		cleanupTables:          usecases.NewCleanupOnboardingTables(cleanupRepo, deps.runtimeCfg.MetaRetention, o11y),
		recordJourneyTimestamp: usecases.NewRecordJourneyTimestamp(magicTokenRepo, o11y),
	}, nil
}

func buildNotificationChannelGateway(
	whatsAppGateway appinterfaces.WhatsAppGateway,
) appinterfaces.OutreachChannelGateway {
	whatsAppSender := adapters.NewWhatsAppSender(whatsAppGateway)
	channelSenders := map[string]notification.ChannelSenders{
		notification.ChannelWhatsApp: whatsAppSender.AsChannelSenders(),
	}
	return notification.NewMultiChannelGateway(channelSenders)
}

func newWhatsAppMessageProcessor(
	useCases onboardingUseCasesBundle,
	deps onboardingDependencies,
	o11y observability.Observability,
) *services.WhatsAppMessageProcessor {
	return services.NewWhatsAppMessageProcessor(
		useCases.consumeToken,
		useCases.fallbackActivation,
		deps.whatsAppGateway,
		deps.runtimeCfg.Messages,
		o11y,
	)
}

func newManagerPublisher(
	db *sqlx.DB,
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	o11y observability.Observability,
) outbox.Publisher {
	return &managerPublisher{db: db, outboxFactory: outboxFactory, cfg: cfg, o11y: o11y}
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
	db *sqlx.DB,
	factory appinterfaces.RepositoryFactory,
	o11y observability.Observability,
) {
	_ = o11y.Metrics().Gauge(
		"onboarding_tokens_paid_unconsumed",
		"Total de tokens no estado PAID ainda nao consumidos",
		"1",
		func(ctx context.Context) float64 {
			repo := factory.MagicTokenRepository(db)
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
	recordJourneyTimestamp *usecases.RecordJourneyTimestamp,
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
	beaconLimiter := middleware.NewRateLimiter(
		cfg.BeaconRateLimitPerMin,
		cfg.BeaconRateLimitBurst,
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
	beaconHandler := handlers.NewRecordJourneyBeaconHandler(recordJourneyTimestamp, o11y)

	return onboardingserver.NewPublicRouter(
		checkoutHandler,
		stateHandler,
		beaconHandler,
		checkoutLimiter,
		stateLimiter,
		beaconLimiter,
		runtimeCfg.CheckoutCORSOrigins,
	)
}

func (p *managerPublisher) Publish(ctx context.Context, evt outbox.Event) error {
	db := p.db
	if tx, ok := database.FromContext(ctx); ok {
		db = tx
	}
	storage := p.outboxFactory.OutboxRepository(db)
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
