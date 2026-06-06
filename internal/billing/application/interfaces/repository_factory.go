package interfaces

import "github.com/JailtonJunior94/devkit-go/pkg/database"

type RepositoryFactory interface {
	SubscriptionRepository(db database.DBTX) SubscriptionRepository
	ProcessedEventRepository(db database.DBTX) ProcessedEventRepository
	KiwifyEventRepository(db database.DBTX) KiwifyEventRepository
	PlanRepository(db database.DBTX) PlanRepository
	ReconciliationCheckpointRepository(db database.DBTX) ReconciliationCheckpointRepository
}
