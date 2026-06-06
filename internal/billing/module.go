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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
	billingserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/handlers"
	billingjobs "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/jobs/handlers"
	billingmessaging "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging"
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
	SubscriptionEventPublisher *producers.SubscriptionEventPublisher
	EventHandlers              []EventHandlerRegistration
}

func NewBillingModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) (BillingModule, error) {
	factory := repositories.NewRepositoryFactory(o11y)
	outboxFactory := outbox.NewRepositoryFactory(o11y)
	idGen := id.NewUUIDGenerator()

	publisher := producers.NewSubscriptionEventPublisher(outboxFactory, cfg.OutboxConfig, idGen)

	subUoW := uow.New[entities.Subscription](mgr, uow.WithObservability(o11y))

	saleApproved := usecases.NewProcessSaleApproved(subUoW, factory, publisher, o11y)
	subRenewed := usecases.NewProcessSubscriptionRenewed(subUoW, factory, publisher, o11y)
	subLate := usecases.NewProcessSubscriptionLate(subUoW, factory, publisher, o11y)
	subCanceled := usecases.NewProcessSubscriptionCanceled(subUoW, factory, publisher, o11y)
	refund := usecases.NewProcessRefundOrChargeback(subUoW, factory, publisher, o11y)

	kiwifyClient, err := kiwify.NewClient(o11y, kiwify.Config{
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
		return BillingModule{}, fmt.Errorf("billing: criar cliente Kiwify: %w", err)
	}

	db := mgr.DBTX(context.Background())
	productIDs := map[valueobjects.PlanCode]string{
		valueobjects.PlanCodeMonthly:   cfg.KiwifyConfig.ProductIDMonthly,
		valueobjects.PlanCodeQuarterly: cfg.KiwifyConfig.ProductIDQuarterly,
		valueobjects.PlanCodeAnnual:    cfg.KiwifyConfig.ProductIDAnnual,
	}
	if cfg.KiwifyConfig.ProductIDMonthly != "" || cfg.KiwifyConfig.ProductIDQuarterly != "" || cfg.KiwifyConfig.ProductIDAnnual != "" {
		if err := factory.PlanRepository(db).ConfigureProductIDs(context.Background(), productIDs); err != nil {
			return BillingModule{}, fmt.Errorf("billing: configurar IDs de produto Kiwify: %w", err)
		}
	}
	reconcile := usecases.NewReconcileSubscriptions(db, factory, kiwifyClient, saleApproved, refund, o11y)
	notificationSender := billingmessaging.NewNoopNotificationSender()
	processWebhook := usecases.NewProcessKiwifyWebhook(
		saleApproved,
		subRenewed,
		subLate,
		subCanceled,
		refund,
		factory,
		db,
		o11y,
	)
	runReconciliation := usecases.NewRunReconciliation(db, factory, reconcile, o11y)
	cleanupKiwifyEvents := usecases.NewCleanupKiwifyEvents(db, factory, cfg.BillingConfig, o11y)
	sendNotification := usecases.NewSendSubscriptionNotification(notificationSender, o11y)

	webhookHandler := handlers.NewKiwifyWebhookHandler(processWebhook, o11y)

	webhookRouter := billingserver.NewWebhookRouter(webhookHandler, cfg.KiwifyConfig.WebhookSecret, cfg.KiwifyConfig.WebhookSecretNext)

	reconciliationJob := billingjobs.NewReconciliationJob(runReconciliation, cfg.KiwifyConfig)
	housekeepingJob := billingjobs.NewKiwifyEventsHousekeepingJob(cleanupKiwifyEvents, cfg.BillingConfig)

	notificationPastDue := consumers.NewNotificationHandler(sendNotification, producers.EventTypeSubscriptionPastDue, o11y)
	notificationRefunded := consumers.NewNotificationHandler(sendNotification, producers.EventTypeSubscriptionRefunded, o11y)
	notificationExpired := consumers.NewNotificationHandler(sendNotification, producers.EventTypeSubscriptionExpired, o11y)

	return BillingModule{
		RepositoryFactory:          factory,
		WebhookRouter:              webhookRouter,
		ReconciliationJob:          reconciliationJob,
		KiwifyEventsHousekeeper:    housekeepingJob,
		SubscriptionEventPublisher: publisher,
		EventHandlers: []EventHandlerRegistration{
			{EventType: producers.EventTypeSubscriptionPastDue, Handler: notificationPastDue},
			{EventType: producers.EventTypeSubscriptionRefunded, Handler: notificationRefunded},
			{EventType: producers.EventTypeSubscriptionExpired, Handler: notificationExpired},
		},
	}, nil
}
