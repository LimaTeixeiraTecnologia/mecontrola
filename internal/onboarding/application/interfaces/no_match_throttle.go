package interfaces

import (
	"context"
	"time"
)

type NoMatchThrottle interface {
	AllowReply(ctx context.Context, mobileE164 string, windowStart time.Time) (bool, error)
	DeleteBefore(ctx context.Context, before time.Time, batchSize int) (int64, error)
}
