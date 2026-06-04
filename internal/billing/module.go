package billing

import (
	"context"
	"fmt"
	"log/slog"

	chiserver "github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	billinginfra "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure"
	billingcache "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/cache"
	kiwifyclient "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
	billinghttp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server"
	billingoutbox "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/outbox"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories/postgres"
	billingscheduler "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/scheduler"
	identityinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	platformhttpclient "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	platformid "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/runtime"
)

type Ports struct{}

type Module struct {
	Ports   Ports
	routers []chiserver.Router
	runners []runtime.Runner
}

type Option func(*options)

type options struct {
	config         *configs.Config
	foundation     runtime.Foundation
	logger         *slog.Logger
	db             *database.Manager
	provider       *observability.Provider
	userRepository identityinterfaces.UserRepository
}

func WithConfig(config *configs.Config) Option {
	return func(opts *options) {
		opts.config = config
	}
}

func WithFoundation(foundation runtime.Foundation) Option {
	return func(opts *options) {
		opts.foundation = foundation
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(opts *options) {
		opts.logger = logger
	}
}

func WithDatabase(db *database.Manager) Option {
	return func(opts *options) {
		opts.db = db
	}
}

func WithProvider(provider *observability.Provider) Option {
	return func(opts *options) {
		opts.provider = provider
	}
}

func WithUserRepository(userRepository identityinterfaces.UserRepository) Option {
	return func(opts *options) {
		opts.userRepository = userRepository
	}
}

func NewModule(opts ...Option) (*Module, error) {
	settings := options{}
	for _, opt := range opts {
		opt(&settings)
	}

	if settings.config == nil {
		return nil, fmt.Errorf("billing module: config não pode ser nil")
	}
	if settings.db == nil {
		return nil, fmt.Errorf("billing module: database não pode ser nil")
	}
	if settings.provider == nil {
		return nil, fmt.Errorf("billing module: observability não pode ser nil")
	}
	if settings.userRepository == nil {
		return nil, fmt.Errorf("billing module: user repository não pode ser nil")
	}
	if settings.logger == nil {
		settings.logger = slog.Default()
	}
	if settings.foundation.Bus == nil {
		settings.foundation = runtime.NewFoundation()
	}

	webhookRepo := billingrepos.NewPgxWebhookEventRepository(settings.db)
	subscriptionRepo := billingrepos.NewPgxSubscriptionRepository(settings.db)

	adapter, err := newWiring(settings).buildKiwifyAdapter(context.Background(), subscriptionRepo)
	if err != nil {
		return nil, err
	}

	registry := outbox.NewRegistry()
	outboxStorage := outbox.NewPgxStorage(settings.db)
	outboxPublisher := outbox.NewPublisher(outboxStorage, registry, nil)

	ingestUseCase, processUseCase, anonymizeUseCase, reconcileUseCase, err := newWiring(settings).buildUseCases(
		adapter,
		webhookRepo,
		subscriptionRepo,
		outboxPublisher,
	)
	if err != nil {
		return nil, err
	}

	if err := billingoutbox.RegisterHandlers(registry, processUseCase); err != nil {
		return nil, fmt.Errorf("billing module: outbox registry: %w", err)
	}

	outboxMetrics, err := outbox.NewOutboxMetrics(settings.provider.Observability())
	if err != nil {
		return nil, fmt.Errorf("billing module: outbox metrics: %w", err)
	}

	outboxRunner, err := outbox.NewModule(outbox.ModuleDeps{
		Config:     settings.config.OutboxConfig,
		Storage:    outboxStorage,
		Registry:   registry,
		Metrics:    outboxMetrics,
		Logger:     settings.logger,
		InstanceID: outbox.NewInstanceID(),
	})
	if err != nil {
		return nil, fmt.Errorf("billing module: outbox runner: %w", err)
	}

	schedulerRunner := billingscheduler.NewBillingScheduler(billingscheduler.Deps{
		ReconcileUseCase:  reconcileUseCase,
		AnonymizeUseCase:  anonymizeUseCase,
		ReconcileSchedule: settings.config.KiwifyConfig.ReconciliationInterval,
		AnonymizeSchedule: settings.config.BillingConfig.AnonymizationSchedule,
		Logger:            settings.logger,
	})

	handler := billinghttp.NewKiwifyWebhookHandler(
		ingestUseCase,
		settings.logger,
		settings.config.KiwifyConfig.WebhookTokenHeader,
	)

	return &Module{
		routers: []chiserver.Router{billinghttp.NewKiwifyRouteRegistrar(handler)},
		runners: []runtime.Runner{outboxRunner, schedulerRunner},
	}, nil
}

func (m *Module) Routers() []chiserver.Router {
	return m.routers
}

func (m *Module) Runners() []runtime.Runner {
	return m.runners
}

type wiring struct {
	options options
}

func newWiring(options options) *wiring {
	return &wiring{options: options}
}

func (w *wiring) buildKiwifyAdapter(
	ctx context.Context,
	subscriptionRepo *billingrepos.PgxSubscriptionRepository,
) (*kiwifyclient.KiwifyAdapter, error) {
	kiwifyCfg := w.options.config.KiwifyConfig

	platformClient, err := platformhttpclient.NewClient(
		w.options.provider.Observability(),
		platformhttpclient.WithBaseURL(kiwifyCfg.APIBaseURL),
		platformhttpclient.WithTimeout(kiwifyCfg.HTTPTimeout),
		platformhttpclient.WithDefaultRetry(kiwifyCfg.HTTPRetryMaxAttempts, kiwifyCfg.HTTPRetryBackoff),
		platformhttpclient.WithTarget("kiwify"),
	)
	if err != nil {
		return nil, fmt.Errorf("billing module: construir platform httpclient: %w", err)
	}

	kiwifyHTTPClient := kiwifyclient.NewClient(
		platformClient,
		kiwifyCfg.RateLimitMaxRequestsPerMin,
		kiwifyCfg.RateLimitBurst,
	)

	oauthClient := kiwifyclient.NewOAuthClient(
		platformClient,
		kiwifyCfg.ClientID,
		kiwifyCfg.ClientSecret,
		kiwifyCfg.OAuthTokenSafetyMargin,
	)

	verifier := kiwifyclient.NewTokenSignatureVerifier(
		w.options.config.KiwifyConfig.WebhookSecret,
		w.options.config.KiwifyConfig.WebhookTokenHeader,
	)

	plansRegistry, err := kiwifyclient.NewBillingPlansRegistry(ctx, subscriptionRepo)
	if err != nil {
		return nil, fmt.Errorf("billing module: plans registry: %w", err)
	}

	mapper := kiwifyclient.NewPayloadMapper(plansRegistry, nil)
	return kiwifyclient.NewKiwifyAdapter(kiwifyHTTPClient, oauthClient, verifier, mapper, plansRegistry), nil
}

func (w *wiring) buildUseCases(
	adapter *kiwifyclient.KiwifyAdapter,
	webhookRepo *billingrepos.PgxWebhookEventRepository,
	subscriptionRepo *billingrepos.PgxSubscriptionRepository,
	outboxPublisher outbox.Publisher,
) (
	ingest *usecases.IngestKiwifyWebhookUseCase,
	process *usecases.ProcessBillingEventUseCase,
	anonymize *usecases.AnonymizeWebhookEventsUseCase,
	reconcile *usecases.ReconcileSubscriptionsUseCase,
	err error,
) {
	metrics, err := observability.RegisterUsecaseMetrics(w.options.provider.Observability().Metrics())
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("billing module: registrar metrics: %w", err)
	}

	idGenerator := platformid.NewUUIDGenerator()
	userResolver := billinginfra.NewIdentityUserResolverAdapter(w.options.userRepository)

	entitlementCache := billingcache.NewEntitlementLRU(
		w.options.config.BillingConfig.EntitlementCacheCapacity,
		w.options.config.BillingConfig.EntitlementCacheTTL,
	)

	redactor := services.NewPIIRedactor()

	ingest = usecases.NewIngestKiwifyWebhookUseCase(
		adapter,
		webhookRepo,
		outboxPublisher,
		database.NewUnitOfWork[output.IngestWebhookResult](w.options.db),
		idGenerator,
		w.options.provider.Observability(),
		metrics,
	)

	process = usecases.NewProcessBillingEventUseCase(
		webhookRepo,
		subscriptionRepo,
		adapter,
		userResolver,
		entitlementCache,
		w.options.foundation.Bus,
		database.NewUnitOfWork[usecases.ProcessBillingEventResult](w.options.db),
		idGenerator,
		w.options.logger,
		w.options.provider.Observability(),
		metrics,
	)

	anonymize = usecases.NewAnonymizeWebhookEventsUseCase(
		webhookRepo,
		redactor,
		w.options.provider.Observability(),
		metrics,
	)

	reconcile = usecases.NewReconcileSubscriptionsUseCase(
		subscriptionRepo,
		webhookRepo,
		adapter,
		outboxPublisher,
		database.NewUnitOfWork[output.ReconciliationReport](w.options.db),
		idGenerator,
		w.options.provider.Observability(),
		metrics,
		w.options.config.KiwifyConfig.ReconciliationBatchSize,
	)

	return ingest, process, anonymize, reconcile, nil
}
