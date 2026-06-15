package repositories

import (
	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories/postgres"
)

type repositoryFactory struct {
	o11y observability.Observability
}

func NewRepositoryFactory(o11y observability.Observability) interfaces.RepositoryFactory {
	return &repositoryFactory{o11y: o11y}
}

func (r *repositoryFactory) UserRepository(db database.DBTX) interfaces.UserRepository {
	return postgres.NewUserRepository(r.o11y, db)
}

func (r *repositoryFactory) UserIdentityRepository(db database.DBTX) interfaces.UserIdentityRepository {
	return postgres.NewUserIdentityRepository(r.o11y, db)
}

func (r *repositoryFactory) EntitlementRepository(db database.DBTX) interfaces.EntitlementRepository {
	return postgres.NewEntitlementRepository(r.o11y, db)
}

func (r *repositoryFactory) AuthEventsRepository(db database.DBTX) interfaces.AuthEventsRepository {
	return postgres.NewAuthEventsRepository(r.o11y, db)
}

func NewSubscriptionProjectionReader(
	mgr manager.Manager,
	o11y observability.Observability,
) interfaces.SubscriptionProjectionReader {
	return postgres.NewSubscriptionProjectionReader(mgr, o11y)
}
