package interfaces

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

type RepositoryFactory interface {
	TransactionRepository(db database.DBTX) TransactionRepository
	CardInvoiceRepository(db database.DBTX) CardInvoiceRepository
	RecurringTemplateRepository(db database.DBTX) RecurringTemplateRepository
	MonthlySummaryRepository(db database.DBTX) MonthlySummaryRepository
	RecurringMaterializationRepository(db database.DBTX) RecurringMaterializationRepository
}
