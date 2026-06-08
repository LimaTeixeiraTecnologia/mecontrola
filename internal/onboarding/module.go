package onboarding

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	identityinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/checkout"
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
	WhatsAppRouter               *onboardingserver.WhatsAppRouter
	SubscriptionConsumer         events.Handler
	PaidWithoutTokenConsumer     events.Handler
	OutreachJob                  worker.Job
	ExpirationJob                worker.Job
	MetaProcessedMessagesCleanup worker.Job
	EventHandlers                []EventHandlerRegistration
}

type moduleUseCases struct {
	createCheckout         *usecases.CreateCheckoutSession
	markTokenPaid          *usecases.MarkTokenPaid
	consumeToken           *usecases.ConsumeMagicToken
	fallbackActivation     *usecases.TryFallbackActivation
	getTokenState          *usecases.GetTokenState
	sendOutreach           *usecases.SendOutreach
	expireTokens           *usecases.ExpireTokens
	handlePaidWithoutToken *usecases.HandlePaidWithoutToken
	cleanupUC              *usecases.CleanupOnboardingTables
}

func NewOnboardingModule(
	mgr manager.Manager,
	cfg configs.OnboardingConfig,
	waCfg configs.WhatsAppConfig,
	outboxCfg configs.OutboxConfig,
	identityModule identity.IdentityModule,
	o11y observability.Observability,
) (OnboardingModule, error) {
	factory := repositories.NewRepositoryFactory(o11y)
	idGen := id.NewUUIDGenerator()
	publisher := newManagerPublisher(mgr, outbox.NewRepositoryFactory(o11y), outboxCfg)
	identityGW := newIdentityGatewayAdapter(identityModule)
	tokenCipher, err := onboardingcrypto.NewTokenCipher(cfg.TokenEncryptionKey)
	if err != nil {
		return OnboardingModule{}, err
	}
	subscriptionBinder := postgres.NewSubscriptionBinder(o11y, mgr.DBTX(context.Background()))

	waGateway, err := buildWhatsAppGateway(o11y, waCfg)
	if err != nil {
		return OnboardingModule{}, err
	}

	ucs := buildUseCases(mgr, cfg, waCfg, factory, publisher, idGen, waGateway, tokenCipher, identityGW, subscriptionBinder, o11y)
	registerModuleGauge(mgr, factory, o11y)

	subscriptionConsumer := consumers.NewSubscriptionPaidConsumer(ucs.markTokenPaid, o11y)
	paidWithoutTokenConsumer := consumers.NewPaidWithoutTokenConsumer(ucs.handlePaidWithoutToken, o11y)
	publicRouter, whatsAppRouter := buildRouters(mgr, cfg, waCfg, factory, ucs, waGateway, o11y)

	return OnboardingModule{
		PublicRouter:                 publicRouter,
		WhatsAppRouter:               whatsAppRouter,
		SubscriptionConsumer:         subscriptionConsumer,
		PaidWithoutTokenConsumer:     paidWithoutTokenConsumer,
		OutreachJob:                  onboardingjobs.NewOutreachJob(ucs.sendOutreach, cfg.OutreachEnabled),
		ExpirationJob:                onboardingjobs.NewTokenExpirationJob(ucs.expireTokens, cfg.TokenExpirationSchedule),
		MetaProcessedMessagesCleanup: onboardingjobs.NewMetaProcessedMessagesCleanupJob(ucs.cleanupUC, cfg.MetaCleanupSchedule),
		EventHandlers: []EventHandlerRegistration{
			{EventType: "billing.subscription.activated", Handler: subscriptionConsumer},
			{EventType: "billing.subscription.activated_without_token", Handler: paidWithoutTokenConsumer},
		},
	}, nil
}

func buildWhatsAppGateway(o11y observability.Observability, waCfg configs.WhatsAppConfig) (appinterfaces.WhatsAppGateway, error) {
	metaClient, err := meta.NewClient(o11y, meta.Config{
		PhoneNumberID: waCfg.PhoneNumberID,
		AccessToken:   waCfg.AccessToken,
	})
	if err != nil {
		return nil, fmt.Errorf("onboarding: criar cliente meta: %w", err)
	}
	return gateway.NewWhatsAppGateway(metaClient), nil
}

func buildUseCases(
	mgr manager.Manager,
	cfg configs.OnboardingConfig,
	waCfg configs.WhatsAppConfig,
	factory appinterfaces.RepositoryFactory,
	pub outbox.Publisher,
	idGen id.Generator,
	waGateway appinterfaces.WhatsAppGateway,
	tokenCipher appinterfaces.TokenCipher,
	identityGW appinterfaces.IdentityGateway,
	subscriptionBinder appinterfaces.SubscriptionBinder,
	o11y observability.Observability,
) moduleUseCases {
	ttl := time.Duration(cfg.TokenTTLDays) * 24 * time.Hour
	outreachGap := time.Duration(cfg.OutreachGapHours) * time.Hour
	retentionPeriod := time.Duration(cfg.MetaRetentionDays) * 24 * time.Hour
	urlBuilder := checkout.NewKiwifyURLBuilder(parseCheckoutURLs(cfg.KiwifyCheckoutURLs), parseCSV(cfg.KiwifyAllowedHosts))
	checkoutUoW := uow.New[entities.MagicToken](mgr, uow.WithObservability(o11y))
	consumeUoW := uow.New[usecases.ConsumeInternalResult](mgr, uow.WithObservability(o11y))
	return moduleUseCases{
		createCheckout:         usecases.NewCreateCheckoutSession(checkoutUoW, factory, urlBuilder, tokenCipher, idGen, ttl, o11y),
		markTokenPaid:          usecases.NewMarkTokenPaid(mgr, factory, o11y),
		consumeToken:           usecases.NewConsumeMagicToken(consumeUoW, factory, identityGW, subscriptionBinder, pub, idGen, o11y),
		fallbackActivation:     usecases.NewTryFallbackActivation(consumeUoW, factory, identityGW, subscriptionBinder, pub, idGen, o11y),
		getTokenState:          usecases.NewGetTokenState(mgr, factory, waCfg.BotNumberE164, waCfg.BotNumberDisplay, o11y),
		sendOutreach:           usecases.NewSendOutreach(mgr, factory, waGateway, tokenCipher, idGen, waCfg.OutreachTemplateName, outreachGap, o11y),
		expireTokens:           usecases.NewExpireTokens(mgr, factory, idGen, o11y),
		handlePaidWithoutToken: usecases.NewHandlePaidWithoutToken(mgr, factory, idGen, o11y),
		cleanupUC:              usecases.NewCleanupOnboardingTables(mgr.DBTX(context.Background()), factory, retentionPeriod, o11y),
	}
}

func registerModuleGauge(mgr manager.Manager, factory appinterfaces.RepositoryFactory, o11y observability.Observability) {
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

func buildRouters(mgr manager.Manager, cfg configs.OnboardingConfig, waCfg configs.WhatsAppConfig, factory appinterfaces.RepositoryFactory, ucs moduleUseCases, waGateway appinterfaces.WhatsAppGateway, o11y observability.Observability) (*onboardingserver.PublicRouter, *onboardingserver.WhatsAppRouter) {
	trustedProxies := parseCSV(cfg.TrustedProxies)

	checkoutCreatedC := o11y.Metrics().Counter("onboarding_checkout_sessions_created_total", "Total de sessoes de checkout criadas", "1")
	checkoutRateLimitedC := o11y.Metrics().Counter("onboarding_checkout_rate_limited_total", "Total de requisicoes de checkout bloqueadas por rate limit", "1")
	invalidAccessC := o11y.Metrics().Counter("ty_page_invalid_access_total", "Total de acessos invalidos a pagina de obrigado", "1")
	sigInvalidC := o11y.Metrics().Counter("meta_signature_invalid_total", "Total de requisicoes rejeitadas por assinatura Meta invalida", "1")

	checkoutLimiter := middleware.NewRateLimiter(cfg.CheckoutRateLimitPerMin, cfg.CheckoutRateLimitBurst, trustedProxies)
	stateLimiter := middleware.NewRateLimiter(cfg.StateRateLimitPerMin, cfg.StateRateLimitBurst, trustedProxies)

	checkoutHandler := handlers.NewCreateCheckoutHandler(ucs.createCheckout,
		func(planID string) {
			checkoutCreatedC.Add(context.Background(), 1, observability.String("plan_id", planID))
		},
		func() { checkoutRateLimitedC.Add(context.Background(), 1) },
		o11y,
	)
	stateHandler := handlers.NewTokenStateHandler(ucs.getTokenState,
		func(reason string) {
			invalidAccessC.Add(context.Background(), 1, observability.String("reason", reason))
		},
		o11y,
	)

	inboundHandler := handlers.NewWhatsAppInboundHandler(
		ucs.consumeToken, ucs.fallbackActivation, waGateway,
		factory, mgr.DBTX(context.Background()), buildMessagesMap(waCfg), o11y,
	)

	public := onboardingserver.NewPublicRouter(checkoutHandler, stateHandler, checkoutLimiter, stateLimiter, parseCSV(cfg.CheckoutCORSOrigins))
	whatsApp := onboardingserver.NewWhatsAppRouter(
		handlers.NewWhatsAppVerifyHandler(waCfg.VerifyToken, o11y),
		inboundHandler,
		waCfg.AppSecret, waCfg.AppSecretNext,
		func() { sigInvalidC.Add(context.Background(), 1) },
	)
	return public, whatsApp
}

type managerPublisher struct {
	mgr           manager.Manager
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
}

func newManagerPublisher(mgr manager.Manager, outboxFactory outbox.OutboxRepositoryFactory, cfg configs.OutboxConfig) outbox.Publisher {
	return &managerPublisher{mgr: mgr, outboxFactory: outboxFactory, cfg: cfg}
}

func (p *managerPublisher) Publish(ctx context.Context, evt outbox.Event) error {
	storage := p.outboxFactory.OutboxRepository(p.mgr.DBTX(ctx))
	publisher := outbox.NewPostgresPublisher(storage, p.cfg)
	return publisher.Publish(ctx, evt)
}

type identityGatewayAdapter struct {
	identityModule identity.IdentityModule
}

func newIdentityGatewayAdapter(m identity.IdentityModule) appinterfaces.IdentityGateway {
	return &identityGatewayAdapter{identityModule: m}
}

func (a *identityGatewayAdapter) UpsertUserByWhatsApp(ctx context.Context, mobileE164, email string) (appinterfaces.UpsertUserResult, error) {
	result, err := a.identityModule.UpsertUserUseCase.Execute(ctx, identityinput.UpsertUserByWhatsApp{
		WhatsAppNumber: mobileE164,
		Email:          email,
	})
	if err != nil {
		return appinterfaces.UpsertUserResult{}, fmt.Errorf("onboarding: identity gateway: upsert user: %w", err)
	}
	return appinterfaces.UpsertUserResult{UserID: result.ID}, nil
}

func parseCheckoutURLs(raw string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}

func parseCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func buildMessagesMap(cfg configs.WhatsAppConfig) map[string]string {
	return map[string]string{
		"welcome_activated":               cfg.WelcomeActivated,
		"already_active":                  cfg.AlreadyActive,
		"code_already_used_other_account": cfg.CodeAlreadyUsed,
		"payment_still_processing_retry":  cfg.PaymentProcessing,
		"code_expired_contact_support":    cfg.CodeExpired,
		"code_invalid_check_again":        cfg.CodeInvalid,
		"system_unavailable_retry":        cfg.SystemUnavailable,
		"please_use_ativar_command":       cfg.PleaseUseAtivar,
		"invalid_country":                 cfg.InvalidCountry,
	}
}
