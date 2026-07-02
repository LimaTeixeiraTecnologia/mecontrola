package interfaces

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

type RepositoryFactory interface {
	ExpenseRepository(db database.DBTX) ExpenseRepository
	AlertRepository(db database.DBTX) AlertRepository
	BudgetRepository(db database.DBTX) BudgetRepository
	PendingEventRepository(db database.DBTX) PendingEventRepository
	ThresholdStateRepository(db database.DBTX) ThresholdStateRepository
	ThresholdAlertSentRepository(db database.DBTX) ThresholdAlertSentRepository
}
