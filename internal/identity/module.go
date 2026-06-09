package identity

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
	jobhandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup"
	deduppostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
)

type EventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type IdentityModule struct {
	RepositoryFactory          interfaces.RepositoryFactory
	UserRouter                 *server.UserRouter
	UpsertUserUseCase          *usecases.UpsertUserByWhatsApp
	FindUserByIDUseCase        *usecases.FindUserByID
	FindUserByWhatsApp         *usecases.FindUserByWhatsApp
	MarkUserDeleted            *usecases.MarkUserDeleted
	EstablishPrincipal         *usecases.EstablishPrincipal
	EntitlementReader          interfaces.EntitlementReader
	SubscriptionProjector      *consumers.SubscriptionEventProjector
	SubscriptionBoundProjector *consumers.SubscriptionBoundProjector
	AuthEventsConsumer         *consumers.AuthEventsConsumer
	AuthEventsHousekeepingJob  *jobhandlers.AuthEventsHousekeepingJob
	WhatsAppLimiter            *ratelimit.Limiter
	WhatsAppDedupRepository    dedup.MessageRepository
	OutboxPublisher            outbox.Publisher
	EventHandlers              []EventHandlerRegistration
}

func NewIdentityModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) IdentityModule {
	factory := repositories.NewRepositoryFactory(o11y)

	outboxFactory := outbox.NewRepositoryFactory(o11y)
	managerPublisher := newIdentityPublisher(mgr, outboxFactory, cfg.OutboxConfig)

	upsertUoW := uow.New[entities.User](mgr, uow.WithObservability(o11y))
	upsertUC := usecases.NewUpsertUserByWhatsApp(upsertUoW, factory, o11y)

	markDeletedUoW := uow.NewVoid(mgr, uow.WithObservability(o11y))
	markDeletedUC := usecases.NewMarkUserDeleted(markDeletedUoW, factory, managerPublisher, o11y)

	establishUoW := uow.New[usecases.EstablishResult](mgr, uow.WithObservability(o11y))
	establishUC := usecases.NewEstablishPrincipal(establishUoW, factory, managerPublisher, o11y)

	findByIDUC := usecases.NewFindUserByID(mgr, factory, o11y)
	findByWhatsAppUC := usecases.NewFindUserByWhatsApp(mgr, factory, o11y)

	upsertHandler := handlers.NewUpsertUserByWhatsAppHandler(upsertUC, o11y)

	projectionReader := repositories.NewSubscriptionProjectionReader(mgr, o11y)
	projectSubscriptionEvent := usecases.NewProjectSubscriptionEvent(factory, mgr, projectionReader, o11y)
	projector := consumers.NewSubscriptionEventProjector(projectSubscriptionEvent, o11y)

	subscriptionBoundProjector := consumers.NewSubscriptionBoundProjector(projectSubscriptionEvent, o11y)

	projectAuthEventUC := usecases.NewProjectAuthEvent(factory, mgr, o11y)
	anonymizeUserAuthEventsUC := usecases.NewAnonymizeUserAuthEvents(factory, mgr, o11y)
	authEventsConsumer := consumers.NewAuthEventsConsumer(projectAuthEventUC, anonymizeUserAuthEventsUC, o11y)

	cleanupAuthEventsUC := usecases.NewCleanupAuthEvents(mgr, factory, cfg.IdentityConfig, o11y)
	authEventsHousekeeping := jobhandlers.NewAuthEventsHousekeepingJob(cleanupAuthEventsUC, cfg.IdentityConfig)

	waLimiter := ratelimit.New(o11y)
	dedupRepo := deduppostgres.NewMessageRepository(o11y, mgr)

	eventHandlers := []EventHandlerRegistration{
		{EventType: "billing.subscription.activated", Handler: projector},
		{EventType: "billing.subscription.renewed", Handler: projector},
		{EventType: "billing.subscription.past_due", Handler: projector},
		{EventType: "billing.subscription.canceled", Handler: projector},
		{EventType: "billing.subscription.refunded", Handler: projector},
		{EventType: "onboarding.subscription_bound", Handler: subscriptionBoundProjector},
		{EventType: "auth.principal_established", Handler: authEventsConsumer},
		{EventType: "auth.failed", Handler: authEventsConsumer},
		{EventType: "auth.unknown_user", Handler: authEventsConsumer},
		{EventType: "user.deleted", Handler: authEventsConsumer},
	}

	return IdentityModule{
		RepositoryFactory:          factory,
		UserRouter:                 server.NewUserRouter(upsertHandler),
		UpsertUserUseCase:          upsertUC,
		FindUserByIDUseCase:        findByIDUC,
		FindUserByWhatsApp:         findByWhatsAppUC,
		MarkUserDeleted:            markDeletedUC,
		EstablishPrincipal:         establishUC,
		EntitlementReader:          &lazyEntitlementReader{mgr: mgr, factory: factory},
		SubscriptionProjector:      projector,
		SubscriptionBoundProjector: subscriptionBoundProjector,
		AuthEventsConsumer:         authEventsConsumer,
		AuthEventsHousekeepingJob:  authEventsHousekeeping,
		WhatsAppLimiter:            waLimiter,
		WhatsAppDedupRepository:    dedupRepo,
		OutboxPublisher:            managerPublisher,
		EventHandlers:              eventHandlers,
	}
}

type lazyEntitlementReader struct {
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func (r *lazyEntitlementReader) FindByUserID(ctx context.Context, userID string) (interfaces.EntitlementRecord, error) {
	return r.factory.EntitlementRepository(r.mgr.DBTX(ctx)).FindByUserID(ctx, userID)
}

type identityPublisher struct {
	mgr           manager.Manager
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
}

func newIdentityPublisher(mgr manager.Manager, outboxFactory outbox.OutboxRepositoryFactory, cfg configs.OutboxConfig) outbox.Publisher {
	return &identityPublisher{mgr: mgr, outboxFactory: outboxFactory, cfg: cfg}
}

func (p *identityPublisher) Publish(ctx context.Context, evt outbox.Event) error {
	storage := p.outboxFactory.OutboxRepository(p.mgr.DBTX(ctx))
	publisher := outbox.NewPostgresPublisher(storage, p.cfg)
	return publisher.Publish(ctx, evt)
}
