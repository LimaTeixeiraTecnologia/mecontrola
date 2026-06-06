package interfaces

import (
	"context"
	"time"
)

type ReconciliationCheckpointRepository interface {
	Get(ctx context.Context, name string) (time.Time, error)
	Set(ctx context.Context, name string, watermark time.Time) error
}
