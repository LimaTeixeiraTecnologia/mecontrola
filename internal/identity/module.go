package identity

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
	identitymiddleware "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/middleware"
	jobhandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup"
	deduppostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/status"
	statuspostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/status/postgres"
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
	ResolvePrincipalByIdentity *usecases.ResolvePrincipalByIdentity
	LinkChannelToUser          *usecases.LinkChannelToUser
	GatewayAuthMiddleware      func(http.Handler) http.Handler
	EntitlementReader          interfaces.EntitlementReader
	SubscriptionProjector      *consumers.SubscriptionEventProjector
	SubscriptionBoundProjector *consumers.SubscriptionBoundProjector
	AuthEventsConsumer         *consumers.AuthEventsConsumer
	AuthEventsHousekeepingJob  *jobhandlers.AuthEventsHousekeepingJob
	RecordGatewayAuthFailure   *usecases.RecordGatewayAuthFailure
	ResolvePreferredChannel    *usecases.ResolvePreferredChannel
	WhatsAppLimiter            *ratelimit.Limiter
	WhatsAppDedupRepository    dedup.MessageRepository
	WhatsAppMessageStatusRepo  status.MessageStatusRepository
	OutboxPublisher            outbox.Publisher
	EventHandlers              []EventHandlerRegistration
}

type identityModuleBuilder struct {
	cfg  *configs.Config
	o11y observability.Observability
	db   *sqlx.DB
}

type identityPublisher struct {
	db            database.DBTX
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	o11y          observability.Observability
}

type lazyEntitlementReader struct {
	db      database.DBTX
	factory interfaces.RepositoryFactory
}

func NewIdentityModule(cfg *configs.Config, o11y observability.Observability, db *sqlx.DB) (IdentityModule, error) {
	builder := &identityModuleBuilder{
		cfg:  cfg,
		o11y: o11y,
		db:   db,
	}
	return builder.build()
}

func (b *identityModuleBuilder) build() (IdentityModule, error) { //nolint:revive // wiring de módulo; cada statement é injeção de dependência sem lógica
	factory := repositories.NewRepositoryFactory(b.o11y)
	publisher := newIdentityPublisher(b.db, outbox.NewRepositoryFactory(b.o11y), b.cfg.OutboxConfig, b.o11y)

	userRepo := factory.UserRepository(b.db)
	userIdentityRepo := factory.UserIdentityRepository(b.db)
	entitlementRepo := factory.EntitlementRepository(b.db)
	authEventsRepo := factory.AuthEventsRepository(b.db)

	upsertUoW := uow.NewUnitOfWork(b.db)
	markDeletedUoW := uow.NewUnitOfWork(b.db)
	establishUoW := uow.NewUnitOfWork(b.db)
	resolveByIdentityUoW := uow.NewUnitOfWork(b.db)

	upsertUser := usecases.NewUpsertUserByWhatsApp(upsertUoW, factory, b.o11y)
	findUserByID := usecases.NewFindUserByID(userRepo, b.o11y)
	findUserByWhatsApp := usecases.NewFindUserByWhatsApp(userRepo, b.o11y)
	markUserDeleted := usecases.NewMarkUserDeleted(markDeletedUoW, factory, publisher, b.o11y)
	establishPrincipal := usecases.NewEstablishPrincipal(establishUoW, factory, publisher, b.o11y)
	resolveByIdentity := usecases.NewResolvePrincipalByIdentity(resolveByIdentityUoW, factory, b.o11y)
	linkChannelUoW := uow.NewUnitOfWork(b.db)
	linkChannelToUser := usecases.NewLinkChannelToUser(linkChannelUoW, factory, b.o11y)

	projectionReader := repositories.NewSubscriptionProjectionReader(b.db, b.o11y)
	projectSubscriptionEvent := usecases.NewProjectSubscriptionEvent(entitlementRepo, projectionReader, b.o11y)
	subscriptionProjector := consumers.NewSubscriptionEventProjector(projectSubscriptionEvent, b.o11y)
	subscriptionBoundProjector := consumers.NewSubscriptionBoundProjector(projectSubscriptionEvent, b.o11y)

	projectAuthEvent := usecases.NewProjectAuthEvent(authEventsRepo, b.o11y)
	anonymizeUserAuthEvents := usecases.NewAnonymizeUserAuthEvents(authEventsRepo, b.o11y)
	authEventsConsumer := consumers.NewAuthEventsConsumer(projectAuthEvent, anonymizeUserAuthEvents, b.o11y)

	cleanupAuthEvents := usecases.NewCleanupAuthEvents(authEventsRepo, b.cfg.IdentityConfig, b.o11y)
	authEventsHousekeepingJob := jobhandlers.NewAuthEventsHousekeepingJob(cleanupAuthEvents, b.cfg.IdentityConfig)

	resolvePreferredChannel := usecases.NewResolvePreferredChannel(userIdentityRepo, b.o11y)
	recordGatewayAuthFailure := usecases.NewRecordGatewayAuthFailure(publisher, b.o11y)
	gatewayAuthMiddleware, err := NewRequireGatewayAuth(b.cfg.IdentityConfig, recordGatewayAuthFailure, b.o11y)
	if err != nil {
		return IdentityModule{}, err
	}

	whatsAppLimiter := ratelimit.New(b.o11y)
	module := IdentityModule{
		RepositoryFactory:          factory,
		UserRouter:                 server.NewUserRouter(handlers.NewUpsertUserByWhatsAppHandler(upsertUser, b.o11y)),
		UpsertUserUseCase:          upsertUser,
		FindUserByIDUseCase:        findUserByID,
		FindUserByWhatsApp:         findUserByWhatsApp,
		MarkUserDeleted:            markUserDeleted,
		EstablishPrincipal:         establishPrincipal,
		ResolvePrincipalByIdentity: resolveByIdentity,
		LinkChannelToUser:          linkChannelToUser,
		GatewayAuthMiddleware:      gatewayAuthMiddleware,
		EntitlementReader:          &lazyEntitlementReader{db: b.db, factory: factory},
		SubscriptionProjector:      subscriptionProjector,
		SubscriptionBoundProjector: subscriptionBoundProjector,
		AuthEventsConsumer:         authEventsConsumer,
		AuthEventsHousekeepingJob:  authEventsHousekeepingJob,
		RecordGatewayAuthFailure:   recordGatewayAuthFailure,
		ResolvePreferredChannel:    resolvePreferredChannel,
		WhatsAppLimiter:            whatsAppLimiter,
		WhatsAppDedupRepository:    deduppostgres.NewMessageRepository(b.o11y, b.db),
		WhatsAppMessageStatusRepo:  statuspostgres.NewMessageStatusRepository(b.o11y, b.db),
		OutboxPublisher:            publisher,
		EventHandlers: []EventHandlerRegistration{
			{EventType: "billing.subscription.activated", Handler: subscriptionProjector},
			{EventType: "billing.subscription.renewed", Handler: subscriptionProjector},
			{EventType: "billing.subscription.past_due", Handler: subscriptionProjector},
			{EventType: "billing.subscription.canceled", Handler: subscriptionProjector},
			{EventType: "billing.subscription.refunded", Handler: subscriptionProjector},
			{EventType: "onboarding.subscription_bound", Handler: subscriptionBoundProjector},
			{EventType: "auth.principal_established", Handler: authEventsConsumer},
			{EventType: "auth.failed", Handler: authEventsConsumer},
			{EventType: "auth.unknown_user", Handler: authEventsConsumer},
			{EventType: "user.deleted", Handler: authEventsConsumer},
		},
	}

	return module, nil
}

func newIdentityPublisher(db database.DBTX, outboxFactory outbox.OutboxRepositoryFactory, cfg configs.OutboxConfig, o11y observability.Observability) outbox.Publisher {
	return &identityPublisher{db: db, outboxFactory: outboxFactory, cfg: cfg, o11y: o11y}
}

func NewRequireGatewayAuth(cfg configs.IdentityConfig, failureUseCase *usecases.RecordGatewayAuthFailure, o11y observability.Observability) (func(http.Handler) http.Handler, error) {
	current, err := decodeGatewaySecret("current", cfg.GatewaySharedSecretCurrent)
	if err != nil {
		return nil, err
	}
	next, err := decodeGatewaySecret("next", cfg.GatewaySharedSecretNext)
	if err != nil {
		return nil, err
	}
	window := cfg.GatewayAuthWindow
	if window <= 0 {
		window = 60 * time.Second
	}
	deps := identitymiddleware.RequireGatewayAuthDeps{
		Secrets:       services.SecretPair{Current: current, Next: next},
		Window:        window,
		FailureLogger: failureUseCase,
		O11y:          o11y,
	}
	return identitymiddleware.RequireGatewayAuth(deps), nil
}

func decodeGatewaySecret(label string, raw string) ([]byte, error) {
	if raw == "" {
		return nil, nil
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("identity: decode gateway secret %s: %w", label, err)
	}
	return decoded, nil
}

func (r *lazyEntitlementReader) FindByUserID(ctx context.Context, userID string) (interfaces.EntitlementRecord, error) {
	db := r.db
	if tx, ok := database.FromContext(ctx); ok {
		db = tx
	}
	return r.factory.EntitlementRepository(db).FindByUserID(ctx, userID)
}

func (p *identityPublisher) Publish(ctx context.Context, evt outbox.Event) error {
	db := p.db
	if tx, ok := database.FromContext(ctx); ok {
		db = tx
	}
	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)
	return publisher.Publish(ctx, evt)
}
