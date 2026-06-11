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

type moduleBuilder struct {
	cfg       *configs.Config
	o11y      observability.Observability
	mgr       manager.Manager
	factory   interfaces.RepositoryFactory
	publisher outbox.Publisher
}

type moduleRuntime struct {
	upsertUser             *usecases.UpsertUserByWhatsApp
	findUserByID           *usecases.FindUserByID
	findUserByWhatsApp     *usecases.FindUserByWhatsApp
	markUserDeleted        *usecases.MarkUserDeleted
	establishPrincipal     *usecases.EstablishPrincipal
	subscriptionProjector  *consumers.SubscriptionEventProjector
	subscriptionBound      *consumers.SubscriptionBoundProjector
	authEventsConsumer     *consumers.AuthEventsConsumer
	authEventsHousekeeping *jobhandlers.AuthEventsHousekeepingJob
}

type identityPublisher struct {
	mgr           manager.Manager
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
}

type lazyEntitlementReader struct {
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func NewIdentityModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) IdentityModule {
	builder := moduleBuilder{
		cfg:       cfg,
		o11y:      o11y,
		mgr:       mgr,
		factory:   repositories.NewRepositoryFactory(o11y),
		publisher: newIdentityPublisher(mgr, outbox.NewRepositoryFactory(o11y), cfg.OutboxConfig),
	}
	return builder.Build()
}

func newIdentityPublisher(mgr manager.Manager, outboxFactory outbox.OutboxRepositoryFactory, cfg configs.OutboxConfig) outbox.Publisher {
	return &identityPublisher{mgr: mgr, outboxFactory: outboxFactory, cfg: cfg}
}

func (b *moduleBuilder) Build() IdentityModule {
	runtime := b.buildRuntime()
	userRouter := b.buildUserRouter(runtime.upsertUser)
	dedupRepository := deduppostgres.NewMessageRepository(b.o11y, b.mgr)

	return IdentityModule{
		RepositoryFactory:          b.factory,
		UserRouter:                 userRouter,
		UpsertUserUseCase:          runtime.upsertUser,
		FindUserByIDUseCase:        runtime.findUserByID,
		FindUserByWhatsApp:         runtime.findUserByWhatsApp,
		MarkUserDeleted:            runtime.markUserDeleted,
		EstablishPrincipal:         runtime.establishPrincipal,
		EntitlementReader:          &lazyEntitlementReader{mgr: b.mgr, factory: b.factory},
		SubscriptionProjector:      runtime.subscriptionProjector,
		SubscriptionBoundProjector: runtime.subscriptionBound,
		AuthEventsConsumer:         runtime.authEventsConsumer,
		AuthEventsHousekeepingJob:  runtime.authEventsHousekeeping,
		WhatsAppLimiter:            ratelimit.New(b.o11y),
		WhatsAppDedupRepository:    dedupRepository,
		OutboxPublisher:            b.publisher,
		EventHandlers:              b.buildEventHandlers(runtime),
	}
}

func (b *moduleBuilder) buildRuntime() moduleRuntime {
	upsertUoW := uow.New[entities.User](b.mgr, uow.WithObservability(b.o11y))
	markDeletedUoW := uow.NewVoid(b.mgr, uow.WithObservability(b.o11y))
	establishUoW := uow.New[usecases.EstablishResult](b.mgr, uow.WithObservability(b.o11y))

	upsertUser := usecases.NewUpsertUserByWhatsApp(upsertUoW, b.factory, b.o11y)
	markUserDeleted := usecases.NewMarkUserDeleted(markDeletedUoW, b.factory, b.publisher, b.o11y)
	establishPrincipal := usecases.NewEstablishPrincipal(establishUoW, b.factory, b.publisher, b.o11y)
	findUserByID := usecases.NewFindUserByID(b.mgr, b.factory, b.o11y)
	findUserByWhatsApp := usecases.NewFindUserByWhatsApp(b.mgr, b.factory, b.o11y)
	subscriptionProjector, subscriptionBound := b.buildSubscriptionConsumers()
	authEventsConsumer := b.buildAuthEventsConsumer()
	authEventsHousekeeping := b.buildAuthEventsHousekeeping()

	return moduleRuntime{
		upsertUser:             upsertUser,
		findUserByID:           findUserByID,
		findUserByWhatsApp:     findUserByWhatsApp,
		markUserDeleted:        markUserDeleted,
		establishPrincipal:     establishPrincipal,
		subscriptionProjector:  subscriptionProjector,
		subscriptionBound:      subscriptionBound,
		authEventsConsumer:     authEventsConsumer,
		authEventsHousekeeping: authEventsHousekeeping,
	}
}

func (b *moduleBuilder) buildUserRouter(upsertUser *usecases.UpsertUserByWhatsApp) *server.UserRouter {
	upsertHandler := handlers.NewUpsertUserByWhatsAppHandler(upsertUser, b.o11y)
	return server.NewUserRouter(upsertHandler)
}

func (b *moduleBuilder) buildSubscriptionConsumers() (*consumers.SubscriptionEventProjector, *consumers.SubscriptionBoundProjector) {
	projectionReader := repositories.NewSubscriptionProjectionReader(b.mgr, b.o11y)
	projectSubscriptionEvent := usecases.NewProjectSubscriptionEvent(b.factory, b.mgr, projectionReader, b.o11y)
	subscriptionProjector := consumers.NewSubscriptionEventProjector(projectSubscriptionEvent, b.o11y)
	subscriptionBound := consumers.NewSubscriptionBoundProjector(projectSubscriptionEvent, b.o11y)
	return subscriptionProjector, subscriptionBound
}

func (b *moduleBuilder) buildAuthEventsConsumer() *consumers.AuthEventsConsumer {
	projectAuthEvent := usecases.NewProjectAuthEvent(b.factory, b.mgr, b.o11y)
	anonymizeUserAuthEvents := usecases.NewAnonymizeUserAuthEvents(b.factory, b.mgr, b.o11y)
	return consumers.NewAuthEventsConsumer(projectAuthEvent, anonymizeUserAuthEvents, b.o11y)
}

func (b *moduleBuilder) buildAuthEventsHousekeeping() *jobhandlers.AuthEventsHousekeepingJob {
	cleanupAuthEvents := usecases.NewCleanupAuthEvents(b.mgr, b.factory, b.cfg.IdentityConfig, b.o11y)
	return jobhandlers.NewAuthEventsHousekeepingJob(cleanupAuthEvents, b.cfg.IdentityConfig)
}

func (b *moduleBuilder) buildEventHandlers(runtime moduleRuntime) []EventHandlerRegistration {
	return []EventHandlerRegistration{
		{EventType: "billing.subscription.activated", Handler: runtime.subscriptionProjector},
		{EventType: "billing.subscription.renewed", Handler: runtime.subscriptionProjector},
		{EventType: "billing.subscription.past_due", Handler: runtime.subscriptionProjector},
		{EventType: "billing.subscription.canceled", Handler: runtime.subscriptionProjector},
		{EventType: "billing.subscription.refunded", Handler: runtime.subscriptionProjector},
		{EventType: "onboarding.subscription_bound", Handler: runtime.subscriptionBound},
		{EventType: "auth.principal_established", Handler: runtime.authEventsConsumer},
		{EventType: "auth.failed", Handler: runtime.authEventsConsumer},
		{EventType: "auth.unknown_user", Handler: runtime.authEventsConsumer},
		{EventType: "user.deleted", Handler: runtime.authEventsConsumer},
	}
}

func (r *lazyEntitlementReader) FindByUserID(ctx context.Context, userID string) (interfaces.EntitlementRecord, error) {
	return r.factory.EntitlementRepository(r.mgr.DBTX(ctx)).FindByUserID(ctx, userID)
}

func (p *identityPublisher) Publish(ctx context.Context, evt outbox.Event) error {
	storage := p.outboxFactory.OutboxRepository(p.mgr.DBTX(ctx))
	publisher := outbox.NewPostgresPublisher(storage, p.cfg)
	return publisher.Publish(ctx, evt)
}
