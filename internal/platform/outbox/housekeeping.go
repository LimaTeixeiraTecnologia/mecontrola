package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type HousekeepingJob struct {
	uow     uow.UnitOfWork[struct{}]
	factory OutboxRepositoryFactory
	cfg     configs.OutboxConfig
	logger  observability.Logger
}

func NewHousekeepingJob(
	unitOfWork uow.UnitOfWork[struct{}],
	factory OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	logger observability.Logger,
) *HousekeepingJob {
	return &HousekeepingJob{
		uow:     unitOfWork,
		factory: factory,
		cfg:     cfg,
		logger:  logger,
	}
}

func (h *HousekeepingJob) Name() string           { return "outbox-housekeeping" }
func (h *HousekeepingJob) Schedule() string       { return h.cfg.HousekeepingSchedule }
func (h *HousekeepingJob) Timeout() time.Duration { return 5 * time.Minute }

func (h *HousekeepingJob) Run(ctx context.Context) error {
	retention := time.Duration(h.cfg.HousekeepingRetentionDays) * 24 * time.Hour
	var total int64
	for {
		var n int64
		_, err := h.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
			storage := h.factory.OutboxRepository(tx)
			deleted, delErr := storage.DeletePublishedBatch(ctx, retention, 1000)
			if delErr != nil {
				return struct{}{}, fmt.Errorf("outbox: housekeeping: %w", delErr)
			}
			n = deleted
			return struct{}{}, nil
		})
		if err != nil {
			return err
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
