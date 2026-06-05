package outbox

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type reaperRunner struct {
	storage Storage
	cfg     configs.OutboxConfig
	logger  observability.Logger
}

func NewReaperRunner(storage Storage, cfg configs.OutboxConfig, logger observability.Logger) *reaperRunner {
	return &reaperRunner{storage: storage, cfg: cfg, logger: logger}
}

func (r *reaperRunner) RunOnce(ctx context.Context) error {
	n, err := r.storage.ResetStuck(ctx, r.cfg.ReaperStuckAfter)
	if err != nil {
		return fmt.Errorf("outbox: reaper: %w", err)
	}
	if n > 0 {
		r.logger.Info(ctx, "outbox: reaper reset stuck events",
			observability.Int64("count", n),
		)
	}
	return nil
}
