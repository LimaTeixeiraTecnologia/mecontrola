package dedup

import (
	"context"
	"time"
)

type MessageRepository interface {
	InsertIfAbsent(ctx context.Context, wamid string) (inserted bool, err error)
	DeleteProcessedBefore(ctx context.Context, before time.Time, batchSize int) (deleted int64, err error)
}
