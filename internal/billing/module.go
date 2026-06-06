package billing

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
	billingserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/handlers"
	billingjobs "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/jobs/handlers"
	billingmessaging "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging"
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

func NewBillingModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) BillingModule {
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

	kiwifyClient, _ := kiwify.NewClient(o11y, kiwify.Config{
		AccountID:                  cfg.KiwifyConfig.ClientID,
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

	db := mgr.DBTX(context.Background())

	var reconcileClient interfaces.KiwifyClient
	if kiwifyClient != nil {
		reconcileClient = kiwifyClient
	}

	reconcile := usecases.NewReconcileSubscriptions(db, factory, reconcileClient, saleApproved, refund, o11y)

	webhookHandler := handlers.NewKiwifyWebhookHandler(
		saleApproved,
		subRenewed,
		subLate,
		subCanceled,
		refund,
		factory,
		mgr,
		o11y,
	)

	webhookRouter := billingserver.NewWebhookRouter(webhookHandler, cfg.KiwifyConfig.WebhookSecret, cfg.KiwifyConfig.WebhookSecretNext)

	reconciliationJob := billingjobs.NewReconciliationJob(db, factory, reconcile, cfg.KiwifyConfig, o11y)
	housekeepingJob := billingjobs.NewKiwifyEventsHousekeepingJob(db, factory, cfg.BillingConfig, o11y)

	_ = billingmessaging.NewNoopNotificationSender()

	return BillingModule{
		RepositoryFactory:          factory,
		WebhookRouter:              webhookRouter,
		ReconciliationJob:          reconciliationJob,
		KiwifyEventsHousekeeper:    housekeepingJob,
		SubscriptionEventPublisher: publisher,
		EventHandlers:              nil,
	}
}
