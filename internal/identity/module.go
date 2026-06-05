package identity

import (
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
)

type IdentityModule struct {
	RepositoryFactory interfaces.RepositoryFactory
	UserRouter        *server.UserRouter
}

func NewIdentityModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) IdentityModule {
	factory := repositories.NewRepositoryFactory(o11y)

	upsertUoW := uow.New[entities.User](mgr, uow.WithObservability(o11y))
	upsertUC := usecases.NewUpsertUserByWhatsApp(upsertUoW, factory, o11y)

	markDeletedUoW := uow.NewVoid(mgr, uow.WithObservability(o11y))
	markDeletedUC := usecases.NewMarkUserDeleted(markDeletedUoW, factory, o11y)
	_ = markDeletedUC

	upsertHandler := handlers.NewUpsertUserByWhatsAppHandler(upsertUC, o11y)

	return IdentityModule{
		RepositoryFactory: factory,
		UserRouter:        server.NewUserRouter(upsertHandler),
	}
}
