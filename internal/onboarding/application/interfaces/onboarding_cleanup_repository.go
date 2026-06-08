package interfaces

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
)

type OnboardingCleanupRepository interface {
	DeleteMetaProcessedOlderThan(ctx context.Context, before time.Time, limit int) (int64, error)
	DeleteConsumerLookupAttemptsOlderThan(ctx context.Context, before time.Time, limit int) (int64, error)
}

type OnboardingCleanupRepositoryFactory interface {
	OnboardingCleanupRepository(db database.DBTX) OnboardingCleanupRepository
}
