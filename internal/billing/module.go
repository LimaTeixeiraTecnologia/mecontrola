package billing

import (
	"context"
	"fmt"

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

type noopNotificationSender struct{}

func NewBillingModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) (BillingModule, error) {
	factory := repositories.NewRepositoryFactory(o11y)
	publisher := producers.NewSubscriptionEventPublisher(outbox.NewRepositoryFactory(o11y), cfg.OutboxConfig, id.NewUUIDGenerator(), o11y)
	kiwifyDBTX := mgr.DBTX(context.Background())
	kiwifyClient, err := newKiwifyClient(cfg, o11y)
	if err != nil {
		return BillingModule{}, err
	}
	catalog, err := billingconfig.NewPlanCatalog(cfg.KiwifyConfig)
	if err != nil {
		return BillingModule{}, err
	}
	if kiwifyDBTX != nil {
		if err := catalog.Apply(context.Background(), factory.PlanRepository(kiwifyDBTX)); err != nil {
			return BillingModule{}, err
		}
	}

	subscriptionUoW := uow.New[entities.Subscription](mgr, uow.WithObservability(o11y))
	saleApproved := usecases.NewProcessSaleApproved(subscriptionUoW, factory, publisher, o11y)
	subscriptionRenewed := usecases.NewProcessSubscriptionRenewed(subscriptionUoW, factory, publisher, o11y)
	subscriptionLate := usecases.NewProcessSubscriptionLate(subscriptionUoW, factory, publisher, o11y)
	subscriptionCanceled := usecases.NewProcessSubscriptionCanceled(subscriptionUoW, factory, publisher, o11y)
	refundOrChargeback := usecases.NewProcessRefundOrChargeback(subscriptionUoW, factory, publisher, o11y)
	graceExpired := usecases.NewProcessSubscriptionGraceExpired(subscriptionUoW, factory, publisher, o11y)

	reconcileSubscriptions := usecases.NewReconcileSubscriptions(kiwifyDBTX, factory, kiwifyClient, saleApproved, refundOrChargeback, o11y)
	processWebhook := usecases.NewProcessKiwifyWebhook(
		saleApproved,
		subscriptionRenewed,
		subscriptionLate,
		subscriptionCanceled,
		refundOrChargeback,
		factory,
		kiwifyDBTX,
		o11y,
	)
	runReconciliation := usecases.NewRunReconciliation(kiwifyDBTX, factory, reconcileSubscriptions, o11y)
	cleanupKiwifyEvents := usecases.NewCleanupKiwifyEvents(kiwifyDBTX, factory, cfg.BillingConfig, o11y)
	sendNotification := usecases.NewSendSubscriptionNotification(&noopNotificationSender{}, o11y)

	webhookHandler := handlers.NewKiwifyWebhookHandler(processWebhook, o11y)
	webhookRouter := billingserver.NewWebhookRouter(
		webhookHandler,
		cfg.KiwifyConfig.WebhookSecret,
		cfg.KiwifyConfig.WebhookSecretNext,
	)
	notificationPastDue := consumers.NewNotificationHandler(sendNotification, producers.EventTypeSubscriptionPastDue, o11y)
	notificationRefunded := consumers.NewNotificationHandler(sendNotification, producers.EventTypeSubscriptionRefunded, o11y)
	notificationExpired := consumers.NewNotificationHandler(sendNotification, producers.EventTypeSubscriptionExpired, o11y)

	return BillingModule{
		RepositoryFactory:          factory,
		WebhookRouter:              webhookRouter,
		ReconciliationJob:          billingjobs.NewReconciliationJob(runReconciliation, cfg.KiwifyConfig),
		KiwifyEventsHousekeeper:    billingjobs.NewKiwifyEventsHousekeepingJob(cleanupKiwifyEvents, cfg.BillingConfig),
		GraceExpirationJob:         billingjobs.NewGraceExpirationJob(graceExpired, cfg.BillingConfig),
		SubscriptionEventPublisher: publisher,
		EventHandlers: []EventHandlerRegistration{
			{EventType: producers.EventTypeSubscriptionPastDue, Handler: notificationPastDue},
			{EventType: producers.EventTypeSubscriptionRefunded, Handler: notificationRefunded},
			{EventType: producers.EventTypeSubscriptionExpired, Handler: notificationExpired},
		},
	}, nil
}

func newKiwifyClient(cfg *configs.Config, o11y observability.Observability) (*kiwify.Client, error) {
	client, err := kiwify.NewClient(o11y, kiwify.Config{
		AccountID:                  cfg.KiwifyConfig.AccountID,
		ClientID:                   cfg.KiwifyConfig.ClientID,
		ClientSecret:               cfg.KiwifyConfig.ClientSecret,
		APIBaseURL:                 cfg.KiwifyConfig.APIBaseURL,
		OAuthTokenSafetyMargin:     cfg.KiwifyConfig.OAuthTokenSafetyMargin,
		RateLimitMaxRequestsPerMin: cfg.KiwifyConfig.RateLimitMaxRequestsPerMin,
		RateLimitBurst:             cfg.KiwifyConfig.RateLimitBurst,
		HTTPTimeout:                cfg.KiwifyConfig.HTTPTimeout,
		HTTPRetryMaxAttempts:       cfg.KiwifyConfig.HTTPRetryMaxAttempts,
		HTTPRetryBackoff:           cfg.KiwifyConfig.HTTPRetryBackoff,
	})
	if err != nil {
		return nil, fmt.Errorf("billing: criar cliente Kiwify: %w", err)
	}
	return client, nil
}

func (s *noopNotificationSender) NotifyTransition(_ context.Context, _ interfaces.NotificationPayload) error {
	return nil
}
