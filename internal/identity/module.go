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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
)

type EventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type IdentityModule struct {
	RepositoryFactory     interfaces.RepositoryFactory
	UserRouter            *server.UserRouter
	UpsertUserUseCase     *usecases.UpsertUserByWhatsApp
	FindUserByIDUseCase   *usecases.FindUserByID
	FindUserByWhatsApp    *usecases.FindUserByWhatsApp
	MarkUserDeleted       *usecases.MarkUserDeleted
	EntitlementReader     interfaces.EntitlementReader
	SubscriptionProjector *consumers.SubscriptionEventProjector
	EventHandlers         []EventHandlerRegistration
}

func NewIdentityModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) IdentityModule {
	factory := repositories.NewRepositoryFactory(o11y)

	upsertUoW := uow.New[entities.User](mgr, uow.WithObservability(o11y))
	upsertUC := usecases.NewUpsertUserByWhatsApp(upsertUoW, factory, o11y)

	markDeletedUoW := uow.NewVoid(mgr, uow.WithObservability(o11y))
	markDeletedUC := usecases.NewMarkUserDeleted(markDeletedUoW, factory, o11y)

	findByIDUC := usecases.NewFindUserByID(mgr, factory, o11y)
	findByWhatsAppUC := usecases.NewFindUserByWhatsApp(mgr, factory, o11y)

	upsertHandler := handlers.NewUpsertUserByWhatsAppHandler(upsertUC, o11y)

	projectionReader := repositories.NewSubscriptionProjectionReader(mgr, o11y)
	projectSubscriptionEvent := usecases.NewProjectSubscriptionEvent(factory, mgr.DBTX(context.Background()), projectionReader, o11y)
	projector := consumers.NewSubscriptionEventProjector(projectSubscriptionEvent, o11y)

	eventHandlers := []EventHandlerRegistration{
		{EventType: "billing.subscription.activated", Handler: projector},
		{EventType: "billing.subscription.renewed", Handler: projector},
		{EventType: "billing.subscription.past_due", Handler: projector},
		{EventType: "billing.subscription.canceled", Handler: projector},
		{EventType: "billing.subscription.refunded", Handler: projector},
	}

	return IdentityModule{
		RepositoryFactory:     factory,
		UserRouter:            server.NewUserRouter(upsertHandler),
		UpsertUserUseCase:     upsertUC,
		FindUserByIDUseCase:   findByIDUC,
		FindUserByWhatsApp:    findByWhatsAppUC,
		MarkUserDeleted:       markDeletedUC,
		EntitlementReader:     factory.EntitlementRepository(mgr.DBTX(context.Background())),
		SubscriptionProjector: projector,
		EventHandlers:         eventHandlers,
	}
}
