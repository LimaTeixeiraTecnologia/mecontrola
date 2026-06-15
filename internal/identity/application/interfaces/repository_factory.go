package interfaces

import "github.com/JailtonJunior94/devkit-go/pkg/database"

type RepositoryFactory interface {
	UserRepository(db database.DBTX) UserRepository
	UserIdentityRepository(db database.DBTX) UserIdentityRepository
	EntitlementRepository(db database.DBTX) EntitlementRepository
	AuthEventsRepository(db database.DBTX) AuthEventsRepository
}
