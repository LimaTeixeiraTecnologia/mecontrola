package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type housekeepingRunner struct {
	storage Storage
	cfg     configs.OutboxConfig
	logger  observability.Logger
}

func NewHousekeepingRunner(storage Storage, cfg configs.OutboxConfig, logger observability.Logger) *housekeepingRunner {
	return &housekeepingRunner{storage: storage, cfg: cfg, logger: logger}
}

func (h *housekeepingRunner) RunOnce(ctx context.Context) error {
	retention := time.Duration(h.cfg.HousekeepingRetentionDays) * 24 * time.Hour
	var total int64
	for {
		n, err := h.storage.DeletePublishedBatch(ctx, retention, 1000)
		if err != nil {
			return fmt.Errorf("outbox: housekeeping: %w", err)
		}
		total += n
		if n == 0 {
			break
		}
	}
	if total > 0 {
		h.logger.Info(ctx, "outbox: housekeeping deleted published events",
			observability.Int64("count", total),
		)
	}
	return nil
}
