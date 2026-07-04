package repositories

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type repositoryFactory struct {
	o11y observability.Observability
}

func NewRepositoryFactory(o11y observability.Observability) interfaces.RepositoryFactory {
	return &repositoryFactory{o11y: o11y}
}

func (f *repositoryFactory) TransactionRepository(db database.DBTX) interfaces.TransactionRepository {
	return postgres.NewTransactionRepository(f.o11y, db)
}

func (f *repositoryFactory) CardInvoiceRepository(db database.DBTX) interfaces.CardInvoiceRepository {
	return postgres.NewCardInvoiceRepository(f.o11y, db)
}

func (f *repositoryFactory) RecurringTemplateRepository(db database.DBTX) interfaces.RecurringTemplateRepository {
	return postgres.NewRecurringTemplateRepository(f.o11y, db)
}

func (f *repositoryFactory) MonthlySummaryRepository(db database.DBTX) interfaces.MonthlySummaryRepository {
	return postgres.NewMonthlySummaryRepository(f.o11y, db)
}

func (f *repositoryFactory) RecurringMaterializationRepository(db database.DBTX) interfaces.RecurringMaterializationRepository {
	return postgres.NewRecurringMaterializationRepository(f.o11y, db)
}
