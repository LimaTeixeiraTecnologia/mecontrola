package interfaces

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type MagicTokenRepository interface {
	Insert(ctx context.Context, token entities.MagicToken) error
	FindByHash(ctx context.Context, tokenHash []byte) (entities.MagicToken, error)
	FindPaidByMobileForFallback(ctx context.Context, mobileE164 string) (entities.MagicToken, error)
	FindPaidForOutreach(ctx context.Context, olderThan time.Time, limit int) ([]entities.MagicToken, error)
	UpdateMarkPaid(ctx context.Context, token entities.MagicToken) error
	UpdateMarkConsumed(ctx context.Context, token entities.MagicToken) error
	UpdateMarkOutreachSent(ctx context.Context, tokenID string, sentAt time.Time) error
	UpdateMarkOutreachReset(ctx context.Context, tokenID string) error
	UpdateTelegramExternalID(ctx context.Context, tokenID, externalID string) error
	BulkExpire(ctx context.Context, now time.Time, limit int) ([]entities.MagicToken, error)
	CountPaidUnconsumed(ctx context.Context) (int64, error)
}

type MagicTokenRepositoryFactory interface {
	MagicTokenRepository(db database.DBTX) MagicTokenRepository
}
