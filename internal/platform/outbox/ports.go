package outbox

import (
	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

type OutboxRepository interface {
	Storage
}

type OutboxRepositoryFactory interface {
	OutboxRepository(db database.DBTX) OutboxRepository
}
