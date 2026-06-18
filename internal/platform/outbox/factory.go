package outbox

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type repositoryFactory struct {
	o11y observability.Observability
}

func NewRepositoryFactory(o11y observability.Observability) OutboxRepositoryFactory {
	return &repositoryFactory{o11y: o11y}
}

func (f *repositoryFactory) OutboxRepository(db database.DBTX) OutboxRepository {
	return NewPostgresStorage(db)
}
