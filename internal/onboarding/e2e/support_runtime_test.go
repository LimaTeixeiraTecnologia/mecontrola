//go:build e2e

package e2e_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	identityinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/binding"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	onboardingservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/checkout"
	onboardingconfig "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/config"
	onboardingcrypto "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/crypto"
	onboardingemail "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/email"
	onboardingserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server"
	httphandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
	httpmiddleware "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/middleware"
	jobhandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/jobs/handlers"
	consumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories"
	onboardingpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
)

const (
	e2ePlanIDMonthly    = "monthly"
	e2eOrderPrefix      = "e2e-order-"
	e2eCheckoutOrigin   = "https://app.mecontrola.test"
	e2eActivateBaseURL  = "https://app.mecontrola.test/activate"
	e2eTokenCipherKey   = "12345678901234567890123456789012"
	e2eBotNumberE164    = "+5511999991111"
	e2eBotNumberDisplay = "+55 11 99999-1111"
)

type onboardingDependencies struct {
	db                        *sqlx.DB
	o11y                      observability.Observability
	factory                   appinterfaces.RepositoryFactory
	identityModule            identity.IdentityModule
	outboxFactory             outbox.OutboxRepositoryFactory
	outboxCfg                 configs.OutboxConfig
	idGen                     id.Generator
	tokenCipher               appinterfaces.TokenCipher
	checkoutBuilder           appinterfaces.CheckoutURLBuilder
	subscriptionBinder        appinterfaces.SubscriptionBinder
	identityGateway           appinterfaces.IdentityGateway
	bindingService            *binding.SubscriptionBindingService
	createCheckout            *usecases.CreateCheckoutSession
	getTokenState             *usecases.GetTokenState
	markTokenPaid             *usecases.MarkTokenPaid
	handlePaidWithoutToken    *usecases.HandlePaidWithoutToken
	sendActivationEmail       *usecases.SendActivationEmail
	consumeToken              *usecases.ConsumeMagicToken
	fallbackActivation        *usecases.TryFallbackActivation
	activateFromInbound       *usecases.ActivateFromInbound
	sendOutreach              *usecases.SendOutreach
	expireTokens              *usecases.ExpireTokens
	cleanupTables             *usecases.CleanupOnboardingTables
	publicRouter              *onboardingserver.PublicRouter
	whatsAppProcessor         *appservices.WhatsAppMessageProcessor
	subscriptionConsumer      events.Handler
	activationEmailConsumer   events.Handler
	paidWithoutTokenConsumer  events.Handler
	activationAttemptConsumer events.Handler
	welcomeConsumer           events.Handler
	outreachJob               worker.Job
	expirationJob             worker.Job
	cleanupJob                worker.Job
	metaGateway               *recordingWhatsAppGateway
	outreachGateway           *recordingOutreachGateway
	emailSender               *recordingEmailSender
}

type txAwarePublisher struct {
	db            *sqlx.DB
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	o11y          observability.Observability
}

type identityGatewayAdapter struct {
	module identity.IdentityModule
}

func buildOnboardingRuntime(t *testing.T, db *sqlx.DB) *onboardingRuntime {
	t.Helper()

	deps := buildOnboardingDependencies(t, db)
	router := chi.NewRouter()
	deps.publicRouter.Register(router)

	return &onboardingRuntime{
		server:          httptest.NewServer(router),
		httpClient:      &http.Client{Timeout: 5 * time.Second},
		deps:            deps,
		metaGateway:     deps.metaGateway,
		outreachGateway: deps.outreachGateway,
		emailSender:     deps.emailSender,
		failingHandler:  &forcedFailureHandler{},
		registryFactory: func() *eventRegistry {
			registry := newEventRegistry()
			mustRegisterHandler(t, registry, "billing.subscription.activated", deps.subscriptionConsumer)
			mustRegisterHandler(t, registry, "billing.subscription.activated", deps.activationEmailConsumer)
			mustRegisterHandler(t, registry, "billing.subscription.activated_without_token", deps.paidWithoutTokenConsumer)
			return registry
		},
		journeyRegistryFactory: func() *eventRegistry {
			registry := newEventRegistry()
			mustRegisterHandler(t, registry, "billing.subscription.activated", deps.subscriptionConsumer)
			mustRegisterHandler(t, registry, "billing.subscription.activated", deps.activationEmailConsumer)
			mustRegisterHandler(t, registry, "billing.subscription.activated_without_token", deps.paidWithoutTokenConsumer)
			mustRegisterHandler(t, registry, "onboarding.activation.attempted.v1", deps.activationAttemptConsumer)
			mustRegisterHandler(t, registry, "onboarding.subscription_bound", deps.welcomeConsumer)
			return registry
		},
	}
}

func buildOnboardingDependencies(t *testing.T, db *sqlx.DB) *onboardingDependencies {
	t.Helper()

	o11y := noop.NewProvider()
	outboxCfg := configs.OutboxConfig{
		DispatcherEnabled:         true,
		DispatcherTickInterval:    time.Second,
		DispatcherBatchSize:       50,
		DispatcherHandlerTimeout:  5 * time.Second,
		RetryMaxAttempts:          3,
		RetryBaseBackoff:          time.Second,
		RetryMaxBackoff:           5 * time.Second,
		HousekeepingRetentionDays: 30,
		HousekeepingSchedule:      "@daily",
		ReaperInterval:            "@every 1m",
		ReaperStuckAfter:          time.Minute,
	}

	cfg := &configs.Config{
		OutboxConfig: outboxCfg,
		IdentityConfig: configs.IdentityConfig{
			AuthEventsHousekeepingSchedule: "@daily",
			AuthEventsHousekeepingBatch:    100,
			AuthEventsRetentionDays:        30,
			GatewaySharedSecretCurrent:     "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1",
			GatewaySharedSecretNext:        "",
			GatewayAuthWindow:              time.Minute,
		},
	}

	identityModule, err := identity.NewIdentityModule(cfg, o11y, db)
	if err != nil {
		t.Fatalf("identity module: %v", err)
	}

	factory := repositories.NewRepositoryFactory(o11y)
	idGen := id.NewUUIDGenerator()
	tokenCipher, err := onboardingcrypto.NewTokenCipher(e2eTokenCipherKey)
	if err != nil {
		t.Fatalf("token cipher: %v", err)
	}

	checkoutBuilder := checkout.NewKiwifyURLBuilder(
		map[string]string{e2ePlanIDMonthly: "https://pay.kiwify.com.br/monthly-checkout"},
		[]string{"pay.kiwify.com.br"},
	)

	metaGateway := &recordingWhatsAppGateway{}
	outreachGateway := &recordingOutreachGateway{}
	emailSender := &recordingEmailSender{}
	publisher := &txAwarePublisher{
		db:            db,
		outboxFactory: outbox.NewRepositoryFactory(o11y),
		cfg:           outboxCfg,
		o11y:          o11y,
	}

	identityGateway := &identityGatewayAdapter{module: identityModule}
	subscriptionBinder := onboardingpostgres.NewSubscriptionBinder(o11y, db)
	bindingService := binding.NewSubscriptionBindingService(
		identityGateway,
		subscriptionBinder,
		onboardingservices.NewMagicTokenWorkflow(),
		publisher,
		idGen,
	)

	runtimeCfg, err := onboardingconfig.NewOnboardingRuntimeConfig(
		configs.OnboardingConfig{
			TokenTTLDays:            7,
			OutreachGapHours:        24,
			OutreachEnabled:         true,
			CheckoutCORSOrigins:     e2eCheckoutOrigin,
			CheckoutRateLimitPerMin: 60,
			CheckoutRateLimitBurst:  10,
			StateRateLimitPerMin:    60,
			StateRateLimitBurst:     10,
			KiwifyCheckoutURLs:      e2ePlanIDMonthly + "=https://pay.kiwify.com.br/monthly-checkout",
			KiwifyAllowedHosts:      "pay.kiwify.com.br",
			MetaRetentionDays:       30,
			MetaCleanupSchedule:     "@daily",
			TokenExpirationSchedule: "@daily",
		},
		configs.WhatsAppConfig{
			OutreachTemplateName: "activation_template",
			BotNumberE164:        e2eBotNumberE164,
			BotNumberDisplay:     e2eBotNumberDisplay,
			WelcomeActivated:     "wa-welcome",
			AlreadyActive:        "wa-already-active",
			CodeAlreadyUsed:      "wa-code-used",
			PaymentProcessing:    "wa-processing",
			CodeExpired:          "wa-expired",
			CodeInvalid:          "wa-invalid",
			SystemUnavailable:    "wa-unavailable",
			InvalidCountry:       "wa-invalid-country",
		},
	)
	if err != nil {
		t.Fatalf("runtime config: %v", err)
	}

	createCheckout := usecases.NewCreateCheckoutSession(
		uow.NewUnitOfWork(db),
		factory,
		checkoutBuilder,
		tokenCipher,
		idGen,
		runtimeCfg.TokenTTL,
		o11y,
	)
	getTokenState := usecases.NewGetTokenState(
		factory.MagicTokenRepository(db),
		e2eBotNumberE164,
		e2eBotNumberDisplay,
		o11y,
	)
	markTokenPaid := usecases.NewMarkTokenPaid(
		factory.MagicTokenRepository(db),
		onboardingservices.NewMagicTokenWorkflow(),
		o11y,
	)
	handlePaidWithoutToken := usecases.NewHandlePaidWithoutToken(
		factory.SupportSignalRepository(db),
		idGen,
		o11y,
	)
	sendActivationEmail := usecases.NewSendActivationEmail(
		factory.MagicTokenRepository(db),
		emailSender,
		onboardingemail.NewActivationTemplate(),
		e2eBotNumberE164,
		e2eActivateBaseURL,
		"noreply@mecontrola.test",
		"MeControla",
		"support@mecontrola.test",
		runtimeCfg.TokenTTL,
		onboardingpostgres.NewSubscriptionBoundChecker(o11y, db),
		o11y,
	)
	consumeToken := usecases.NewConsumeMagicToken(
		uow.NewUnitOfWork(db),
		factory,
		bindingService,
		idGen,
		24*time.Hour,
		o11y,
	)
	fallbackActivation := usecases.NewTryFallbackActivation(
		uow.NewUnitOfWork(db),
		factory,
		bindingService,
		o11y,
	)
	noMatchThrottle := onboardingpostgres.NewNoMatchThrottleRepository(o11y, db)
	activateFromInbound := usecases.NewActivateFromInbound(
		uow.NewUnitOfWork(db),
		factory,
		bindingService,
		consumeToken,
		metaGateway,
		noMatchThrottle,
		24*time.Hour,
		"wa-not-found",
		o11y,
	)
	welcomeConsumer := consumers.NewWelcomeConsumer(
		metaGateway,
		onboardingpostgres.NewWelcomeDedupRepository(o11y, db),
		"wa-welcome",
		"wa-onboarding-intro",
		24*time.Hour,
		o11y,
	)
	activationAttemptConsumer := consumers.NewActivationAttemptConsumer(activateFromInbound, o11y)
	sendOutreach := usecases.NewSendOutreach(
		factory.MagicTokenRepository(db),
		outreachGateway,
		tokenCipher,
		idGen,
		"activation_template",
		runtimeCfg.OutreachGap,
		o11y,
	)
	expireTokens := usecases.NewExpireTokens(db, factory, idGen, o11y)
	cleanupTables := usecases.NewCleanupOnboardingTables(
		factory.OnboardingCleanupRepository(db),
		runtimeCfg.MetaRetention,
		o11y,
	)

	checkoutLimiter := httpmiddleware.NewRateLimiter(60, 10, nil)
	stateLimiter := httpmiddleware.NewRateLimiter(60, 10, nil)
	beaconLimiter := httpmiddleware.NewRateLimiter(60, 10, nil)
	recordJourneyTimestamp := usecases.NewRecordJourneyTimestamp(factory.MagicTokenRepository(db), o11y)
	publicRouter := onboardingserver.NewPublicRouter(
		httphandlers.NewCreateCheckoutHandler(createCheckout, func(string) {}, func() {}, o11y),
		httphandlers.NewTokenStateHandler(getTokenState, func(string) {}, o11y),
		httphandlers.NewRecordJourneyBeaconHandler(recordJourneyTimestamp, o11y),
		checkoutLimiter,
		stateLimiter,
		beaconLimiter,
		[]string{e2eCheckoutOrigin},
	)

	whatsAppProcessor := appservices.NewWhatsAppMessageProcessor(
		consumeToken,
		fallbackActivation,
		metaGateway,
		runtimeCfg.Messages,
		o11y,
	)
	return &onboardingDependencies{
		db:                        db,
		o11y:                      o11y,
		factory:                   factory,
		identityModule:            identityModule,
		outboxFactory:             outbox.NewRepositoryFactory(o11y),
		outboxCfg:                 outboxCfg,
		idGen:                     idGen,
		tokenCipher:               tokenCipher,
		checkoutBuilder:           checkoutBuilder,
		subscriptionBinder:        subscriptionBinder,
		identityGateway:           identityGateway,
		bindingService:            bindingService,
		createCheckout:            createCheckout,
		getTokenState:             getTokenState,
		markTokenPaid:             markTokenPaid,
		handlePaidWithoutToken:    handlePaidWithoutToken,
		sendActivationEmail:       sendActivationEmail,
		consumeToken:              consumeToken,
		fallbackActivation:        fallbackActivation,
		activateFromInbound:       activateFromInbound,
		sendOutreach:              sendOutreach,
		expireTokens:              expireTokens,
		cleanupTables:             cleanupTables,
		publicRouter:              publicRouter,
		whatsAppProcessor:         whatsAppProcessor,
		subscriptionConsumer:      consumers.NewSubscriptionPaidConsumer(markTokenPaid, o11y),
		activationEmailConsumer:   consumers.NewActivationEmailConsumer(sendActivationEmail, o11y),
		paidWithoutTokenConsumer:  consumers.NewPaidWithoutTokenConsumer(handlePaidWithoutToken, o11y),
		activationAttemptConsumer: activationAttemptConsumer,
		welcomeConsumer:           welcomeConsumer,
		outreachJob:               jobhandlers.NewOutreachJob(sendOutreach, true),
		expirationJob:             jobhandlers.NewTokenExpirationJob(expireTokens, "@daily"),
		cleanupJob:                jobhandlers.NewMetaProcessedMessagesCleanupJob(cleanupTables, "@daily"),
		metaGateway:               metaGateway,
		outreachGateway:           outreachGateway,
		emailSender:               emailSender,
	}
}

func mustRegisterHandler(t *testing.T, registry *eventRegistry, eventType string, handler events.Handler) {
	t.Helper()
	if err := registry.Register(eventType, handler); err != nil {
		t.Fatalf("register handler %s: %v", eventType, err)
	}
}

func (p *txAwarePublisher) Publish(ctx context.Context, evt outbox.Event) error {
	dbtx := database.DBTX(p.db)
	if tx, ok := database.FromContext(ctx); ok {
		dbtx = tx
	}
	storage := p.outboxFactory.OutboxRepository(dbtx)
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)
	return publisher.Publish(ctx, evt)
}

func (a *identityGatewayAdapter) UpsertUserByWhatsApp(ctx context.Context, mobileE164, email string) (appinterfaces.UpsertUserResult, error) {
	result, err := a.module.UpsertUserUseCase.Execute(ctx, identityinput.UpsertUserByWhatsApp{
		WhatsAppNumber: mobileE164,
		Email:          email,
	})
	if err != nil {
		return appinterfaces.UpsertUserResult{}, err
	}
	return appinterfaces.UpsertUserResult{UserID: result.ID}, nil
}

const e2eCardClosingOffsetDays = 10

type recordingWhatsAppGateway struct {
	mu       sync.Mutex
	messages []sentWhatsAppMessage
	err      error
}

type sentWhatsAppMessage struct {
	To   string
	Text string
}

func (g *recordingWhatsAppGateway) SendActivationTemplate(_ context.Context, toE164, templateName, token string) (string, error) {
	if g.err != nil {
		return "", g.err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.messages = append(g.messages, sentWhatsAppMessage{
		To:   toE164,
		Text: templateName + ":" + token,
	})
	return "wamid-template", nil
}

func (g *recordingWhatsAppGateway) SendTextMessage(_ context.Context, toE164, text string) error {
	if g.err != nil {
		return g.err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.messages = append(g.messages, sentWhatsAppMessage{To: toE164, Text: text})
	return nil
}

func (g *recordingWhatsAppGateway) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.messages = nil
	g.err = nil
}

type recordingOutreachGateway struct {
	mu            sync.Mutex
	templatesSent []sentOutreachTemplate
	textsSent     []sentOutreachText
	templateErr   error
	textErr       error
}

type sentOutreachTemplate struct {
	Channel      string
	ExternalID   string
	TemplateName string
	Token        string
}

type sentOutreachText struct {
	Channel    string
	ExternalID string
	Text       string
}

func (g *recordingOutreachGateway) SendActivationTemplate(_ context.Context, channel, externalID, templateName, token string) (string, error) {
	if g.templateErr != nil {
		return "", g.templateErr
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.templatesSent = append(g.templatesSent, sentOutreachTemplate{
		Channel:      channel,
		ExternalID:   externalID,
		TemplateName: templateName,
		Token:        token,
	})
	return "template-msg-id", nil
}

func (g *recordingOutreachGateway) SendText(_ context.Context, channel, externalID, text string) error {
	if g.textErr != nil {
		return g.textErr
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.textsSent = append(g.textsSent, sentOutreachText{
		Channel:    channel,
		ExternalID: externalID,
		Text:       text,
	})
	return nil
}

func (g *recordingOutreachGateway) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.templatesSent = nil
	g.textsSent = nil
	g.templateErr = nil
	g.textErr = nil
}

type recordingEmailSender struct {
	mu       sync.Mutex
	messages []appinterfaces.EmailMessage
	err      error
}

func (s *recordingEmailSender) Send(_ context.Context, msg appinterfaces.EmailMessage) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
	return nil
}

func (s *recordingEmailSender) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
	s.err = nil
}

type eventRegistry struct {
	dispatcher events.Dispatcher
}

func newEventRegistry() *eventRegistry {
	return &eventRegistry{dispatcher: events.NewDispatcher()}
}

func (r *eventRegistry) Register(eventType string, handler events.Handler) error {
	return r.dispatcher.Register(eventType, handler)
}

func (r *eventRegistry) HandlersOf(eventType string) []events.Handler {
	return r.dispatcher.HandlersOf(eventType)
}

type forcedFailureHandler struct {
	mu      sync.Mutex
	enabled bool
	err     error
}

func (h *forcedFailureHandler) Handle(context.Context, events.Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.enabled {
		return nil
	}
	if h.err != nil {
		return h.err
	}
	return errors.New("forced handler failure")
}

func (h *forcedFailureHandler) configure(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = true
	h.err = err
}

func (h *forcedFailureHandler) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = false
	h.err = nil
}

func newDeterministicToken(seed byte) string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = seed
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func insertOutboxEnvelope(
	ctx context.Context,
	db *sqlx.DB,
	cfg configs.OutboxConfig,
	o11y observability.Observability,
	eventType string,
	aggregateType string,
	aggregateID string,
	aggregateUserID string,
	payload any,
) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	event, err := outbox.NewEvent(outbox.EventInput{
		Type:            eventType,
		AggregateType:   aggregateType,
		AggregateID:     aggregateID,
		AggregateUserID: aggregateUserID,
		Payload:         raw,
		OccurredAt:      time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	storage := outbox.NewRepositoryFactory(o11y).OutboxRepository(db)
	publisher := outbox.NewObservablePostgresPublisher(storage, cfg, o11y)
	if err := publisher.Publish(ctx, event); err != nil {
		return "", err
	}
	return event.ID, nil
}

func newDispatcherJob(db *sqlx.DB, cfg configs.OutboxConfig, registry outbox.Registry, o11y observability.Observability) *outbox.DispatcherJob {
	return outbox.NewDispatcherJob(
		uow.NewUnitOfWork(db),
		outbox.NewRepositoryFactory(o11y),
		registry,
		cfg,
		o11y.Logger(),
		rand.New(rand.NewSource(42)),
	)
}
