package billing

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	billingconfig "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/config"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
	billingserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/handlers"
	billingjobs "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/producers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type EventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type BillingModule struct {
	RepositoryFactory          interfaces.RepositoryFactory
	WebhookRouter              *billingserver.WebhookRouter
	ReconciliationJob          *billingjobs.ReconciliationJob
	KiwifyEventsHousekeeper    *billingjobs.KiwifyEventsHousekeepingJob
	GraceExpirationJob         *billingjobs.GraceExpirationJob
	SubscriptionEventPublisher *producers.SubscriptionEventPublisher
	EventHandlers              []EventHandlerRegistration
}

type moduleBuilder struct {
	cfg        *configs.Config
	o11y       observability.Observability
	mgr        manager.Manager
	factory    interfaces.RepositoryFactory
	publisher  *producers.SubscriptionEventPublisher
	kiwifyDBTX database.DBTX
}

type moduleRuntime struct {
	webhookRouter        *billingserver.WebhookRouter
	reconciliationJob    *billingjobs.ReconciliationJob
	housekeepingJob      *billingjobs.KiwifyEventsHousekeepingJob
	graceExpirationJob   *billingjobs.GraceExpirationJob
	notificationPastDue  events.Handler
	notificationRefunded events.Handler
	notificationExpired  events.Handler
}

type noopNotificationSender struct{}

func NewBillingModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) (BillingModule, error) {
	factory := repositories.NewRepositoryFactory(o11y)
	publisher := producers.NewSubscriptionEventPublisher(outbox.NewRepositoryFactory(o11y), cfg.OutboxConfig, id.NewUUIDGenerator(), o11y)
	builder := moduleBuilder{
		cfg:        cfg,
		o11y:       o11y,
		mgr:        mgr,
		factory:    factory,
		publisher:  publisher,
		kiwifyDBTX: mgr.DBTX(context.Background()),
	}
	return builder.Build()
}

func (b *moduleBuilder) Build() (BillingModule, error) {
	runtime, err := b.buildRuntime()
	if err != nil {
		return BillingModule{}, err
	}

	return BillingModule{
		RepositoryFactory:          b.factory,
		WebhookRouter:              runtime.webhookRouter,
		ReconciliationJob:          runtime.reconciliationJob,
		KiwifyEventsHousekeeper:    runtime.housekeepingJob,
		GraceExpirationJob:         runtime.graceExpirationJob,
		SubscriptionEventPublisher: b.publisher,
		EventHandlers:              b.buildEventHandlers(runtime),
	}, nil
}

func (b *moduleBuilder) buildRuntime() (moduleRuntime, error) {
	kiwifyClient, err := b.buildKiwifyClient()
	if err != nil {
		return moduleRuntime{}, err
	}
	catalog, err := billingconfig.NewPlanCatalog(b.cfg.KiwifyConfig)
	if err != nil {
		return moduleRuntime{}, err
	}
	if b.kiwifyDBTX != nil {
		if err := catalog.Apply(context.Background(), b.factory.PlanRepository(b.kiwifyDBTX)); err != nil {
			return moduleRuntime{}, err
		}
	}

	subscriptionUoW := uow.New[entities.Subscription](b.mgr, uow.WithObservability(b.o11y))
	saleApproved := usecases.NewProcessSaleApproved(subscriptionUoW, b.factory, b.publisher, b.o11y)
	subscriptionRenewed := usecases.NewProcessSubscriptionRenewed(subscriptionUoW, b.factory, b.publisher, b.o11y)
	subscriptionLate := usecases.NewProcessSubscriptionLate(subscriptionUoW, b.factory, b.publisher, b.o11y)
	subscriptionCanceled := usecases.NewProcessSubscriptionCanceled(subscriptionUoW, b.factory, b.publisher, b.o11y)
	refundOrChargeback := usecases.NewProcessRefundOrChargeback(subscriptionUoW, b.factory, b.publisher, b.o11y)
	graceExpired := usecases.NewProcessSubscriptionGraceExpired(subscriptionUoW, b.factory, b.publisher, b.o11y)

	reconcileSubscriptions := usecases.NewReconcileSubscriptions(b.kiwifyDBTX, b.factory, kiwifyClient, saleApproved, refundOrChargeback, b.o11y)
	processWebhook := usecases.NewProcessKiwifyWebhook(
		saleApproved,
		subscriptionRenewed,
		subscriptionLate,
		subscriptionCanceled,
		refundOrChargeback,
		b.factory,
		b.kiwifyDBTX,
		b.o11y,
	)
	runReconciliation := usecases.NewRunReconciliation(b.kiwifyDBTX, b.factory, reconcileSubscriptions, b.o11y)
	cleanupKiwifyEvents := usecases.NewCleanupKiwifyEvents(b.kiwifyDBTX, b.factory, b.cfg.BillingConfig, b.o11y)
	sendNotification := usecases.NewSendSubscriptionNotification(&noopNotificationSender{}, b.o11y)

	webhookHandler := handlers.NewKiwifyWebhookHandler(processWebhook, b.o11y)
	webhookRouter := billingserver.NewWebhookRouter(
		webhookHandler,
		b.cfg.KiwifyConfig.WebhookSecret,
		b.cfg.KiwifyConfig.WebhookSecretNext,
	)

	return moduleRuntime{
		webhookRouter:        webhookRouter,
		reconciliationJob:    billingjobs.NewReconciliationJob(runReconciliation, b.cfg.KiwifyConfig),
		housekeepingJob:      billingjobs.NewKiwifyEventsHousekeepingJob(cleanupKiwifyEvents, b.cfg.BillingConfig),
		graceExpirationJob:   billingjobs.NewGraceExpirationJob(graceExpired, b.cfg.BillingConfig),
		notificationPastDue:  consumers.NewNotificationHandler(sendNotification, producers.EventTypeSubscriptionPastDue, b.o11y),
		notificationRefunded: consumers.NewNotificationHandler(sendNotification, producers.EventTypeSubscriptionRefunded, b.o11y),
		notificationExpired:  consumers.NewNotificationHandler(sendNotification, producers.EventTypeSubscriptionExpired, b.o11y),
	}, nil
}

func (b *moduleBuilder) buildKiwifyClient() (*kiwify.Client, error) {
	client, err := kiwify.NewClient(b.o11y, kiwify.Config{
		AccountID:                  b.cfg.KiwifyConfig.AccountID,
		ClientID:                   b.cfg.KiwifyConfig.ClientID,
		ClientSecret:               b.cfg.KiwifyConfig.ClientSecret,
		APIBaseURL:                 b.cfg.KiwifyConfig.APIBaseURL,
		OAuthTokenSafetyMargin:     b.cfg.KiwifyConfig.OAuthTokenSafetyMargin,
		RateLimitMaxRequestsPerMin: b.cfg.KiwifyConfig.RateLimitMaxRequestsPerMin,
		RateLimitBurst:             b.cfg.KiwifyConfig.RateLimitBurst,
		HTTPTimeout:                b.cfg.KiwifyConfig.HTTPTimeout,
		HTTPRetryMaxAttempts:       b.cfg.KiwifyConfig.HTTPRetryMaxAttempts,
		HTTPRetryBackoff:           b.cfg.KiwifyConfig.HTTPRetryBackoff,
	})
	if err != nil {
		return nil, fmt.Errorf("billing: criar cliente Kiwify: %w", err)
	}
	return client, nil
}

func (b *moduleBuilder) buildEventHandlers(runtime moduleRuntime) []EventHandlerRegistration {
	return []EventHandlerRegistration{
		{EventType: producers.EventTypeSubscriptionPastDue, Handler: runtime.notificationPastDue},
		{EventType: producers.EventTypeSubscriptionRefunded, Handler: runtime.notificationRefunded},
		{EventType: producers.EventTypeSubscriptionExpired, Handler: runtime.notificationExpired},
	}
}

func (s *noopNotificationSender) NotifyTransition(_ context.Context, _ interfaces.NotificationPayload) error {
	return nil
}
