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

func NewDecisionRepositoryFactory(o11y observability.Observability) interfaces.AgentDecisionRepositoryFactory {
	return &repositoryFactory{o11y: o11y}
}

func NewWorkingMemoryRepositoryFactory(o11y observability.Observability) interfaces.WorkingMemoryRepositoryFactory {
	return &repositoryFactory{o11y: o11y}
}

func NewObservationRepositoryFactory(o11y observability.Observability) interfaces.ObservationRepositoryFactory {
	return &repositoryFactory{o11y: o11y}
}

func (f *repositoryFactory) AgentSessionRepository(db database.DBTX) interfaces.AgentSessionRepository {
	return postgres.NewAgentSessionRepository(f.o11y, db)
}

func (f *repositoryFactory) AgentDecisionRepository(db database.DBTX) interfaces.AgentDecisionRepository {
	return postgres.NewAgentDecisionRepository(f.o11y, db)
}

func (f *repositoryFactory) WorkingMemoryRepository(db database.DBTX) interfaces.WorkingMemoryRepository {
	return postgres.NewWorkingMemoryRepository(f.o11y, db)
}

func (f *repositoryFactory) ObservationRepository(db database.DBTX) interfaces.ObservationRepository {
	return postgres.NewObservationRepository(f.o11y, db)
}
