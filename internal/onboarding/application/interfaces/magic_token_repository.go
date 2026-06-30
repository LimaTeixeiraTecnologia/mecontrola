package interfaces

import (
	"context"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type MagicTokenRepository interface {
	Insert(ctx context.Context, token entities.MagicToken) error
	FindByHash(ctx context.Context, tokenHash []byte) (entities.MagicToken, error)
	FindPaidByMobileForFallback(ctx context.Context, mobileE164 string) (entities.MagicToken, error)
	FindActivableByMobile(ctx context.Context, mobileE164 string, paidAfter time.Time) (entities.MagicToken, error)
	HasConsumedByMobile(ctx context.Context, mobileE164 string) (bool, error)
	FindPaidForOutreach(ctx context.Context, olderThan time.Time, limit int) ([]entities.MagicToken, error)
	UpdateMarkPaid(ctx context.Context, token entities.MagicToken) error
	UpdateMarkConsumed(ctx context.Context, token entities.MagicToken) error
	UpdateMarkOutreachSent(ctx context.Context, tokenID string, sentAt time.Time) error
	UpdateMarkOutreachReset(ctx context.Context, tokenID string) error
	UpdateSetEmailSentAt(ctx context.Context, tokenID string, now time.Time) error
	IsEmailSent(ctx context.Context, tokenID string) (bool, error)
	UpdateMarkActivationStartedAt(ctx context.Context, tokenID string, now time.Time) error
	BulkExpire(ctx context.Context, now time.Time, limit int) ([]entities.MagicToken, error)
	CountPaidUnconsumed(ctx context.Context) (int64, error)
	MarkPageOpened(ctx context.Context, tokenID string, now time.Time) error
	MarkWhatsAppOpened(ctx context.Context, tokenID string, now time.Time) error
}

type MagicTokenRepositoryFactory interface {
	MagicTokenRepository(db database.DBTX) MagicTokenRepository
}
