package dedup

import (
	"context"
	"time"
)

type MessageRepository interface {
	InsertIfAbsent(ctx context.Context, wamid string) (inserted bool, err error)
	DeleteProcessedBefore(ctx context.Context, before time.Time, batchSize int) (deleted int64, err error)
}

type ConsumerMessageRepository interface {
	DeleteProcessedBefore(ctx context.Context, cutoff time.Time) (deleted int64, err error)
}
