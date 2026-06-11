package repositories

import (
	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories/postgres"
)

type repositoryFactory struct {
	o11y observability.Observability
}

func NewRepositoryFactory(o11y observability.Observability) interfaces.RepositoryFactory {
	return &repositoryFactory{o11y: o11y}
}

func (f *repositoryFactory) ExpenseRepository(db database.DBTX) interfaces.ExpenseRepository {
	return postgres.NewExpenseRepository(f.o11y, db)
}

func (f *repositoryFactory) AlertRepository(db database.DBTX) interfaces.AlertRepository {
	return postgres.NewAlertRepository(f.o11y, db)
}

func (f *repositoryFactory) BudgetRepository(db database.DBTX) interfaces.BudgetRepository {
	return postgres.NewBudgetRepository(f.o11y, db)
}

func (f *repositoryFactory) PendingEventRepository(db database.DBTX) interfaces.PendingEventRepository {
	return postgres.NewPendingEventRepository(f.o11y, db)
}

func (f *repositoryFactory) ThresholdStateRepository(db database.DBTX) interfaces.ThresholdStateRepository {
	return postgres.NewThresholdStateRepository(f.o11y, db)
}
