package repositories

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type repositoryFactory struct {
	o11y observability.Observability
}

func NewRepositoryFactory(o11y observability.Observability) interfaces.AgentSessionRepositoryFactory {
	return &repositoryFactory{o11y: o11y}
}

func (f *repositoryFactory) AgentSessionRepository(db database.DBTX) interfaces.AgentSessionRepository {
	return postgres.NewAgentSessionRepository(f.o11y, db)
}
