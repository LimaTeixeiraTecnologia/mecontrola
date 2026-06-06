package repositories

import (
	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories/postgres"
)

type repositoryFactory struct {
	o11y observability.Observability
}

func NewRepositoryFactory(o11y observability.Observability) interfaces.RepositoryFactory {
	return &repositoryFactory{o11y: o11y}
}

func (f *repositoryFactory) SubscriptionRepository(db database.DBTX) interfaces.SubscriptionRepository {
	return postgres.NewSubscriptionRepository(f.o11y, db)
}

func (f *repositoryFactory) ProcessedEventRepository(db database.DBTX) interfaces.ProcessedEventRepository {
	return postgres.NewProcessedEventRepository(f.o11y, db)
}

func (f *repositoryFactory) KiwifyEventRepository(db database.DBTX) interfaces.KiwifyEventRepository {
	return postgres.NewKiwifyEventRepository(f.o11y, db)
}

func (f *repositoryFactory) PlanRepository(db database.DBTX) interfaces.PlanRepository {
	return postgres.NewPlanRepository(f.o11y, db)
}

func (f *repositoryFactory) ReconciliationCheckpointRepository(db database.DBTX) interfaces.ReconciliationCheckpointRepository {
	return postgres.NewReconciliationCheckpointRepository(f.o11y, db)
}
