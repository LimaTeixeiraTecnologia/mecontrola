package repositories

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
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
	db *sqlx.DB,
	o11y observability.Observability,
) interfaces.SubscriptionProjectionReader {
	return postgres.NewSubscriptionProjectionReader(db, o11y)
}
