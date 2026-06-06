package interfaces

import "github.com/JailtonJunior94/devkit-go/pkg/database"

type RepositoryFactory interface {
	UserRepository(db database.DBTX) UserRepository
	EntitlementRepository(db database.DBTX) EntitlementRepository
}
