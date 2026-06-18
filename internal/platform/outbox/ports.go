package outbox

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type OutboxRepository interface {
	Storage
}

type OutboxRepositoryFactory interface {
	OutboxRepository(db database.DBTX) OutboxRepository
}
