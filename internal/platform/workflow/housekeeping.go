package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

type HousekeepingJob struct {
	uow     uow.UnitOfWork
	factory StoreFactory
	cfg     configs.WorkflowKernelConfig
	logger  observability.Logger
}

type StoreFactory interface {
	Store(db database.DBTX) Store
}

func NewHousekeepingJob(
	unitOfWork uow.UnitOfWork,
	factory StoreFactory,
	cfg configs.WorkflowKernelConfig,
	logger observability.Logger,
) (*HousekeepingJob, error) {
	if cfg.HousekeepingRetentionDays <= 0 {
		return nil, errors.New("workflow: housekeeping: retention_days must be > 0")
	}
	if cfg.HousekeepingBatchSize <= 0 {
		return nil, errors.New("workflow: housekeeping: batch_size must be > 0")
	}
	return &HousekeepingJob{
		uow:     unitOfWork,
		factory: factory,
		cfg:     cfg,
		logger:  logger,
	}, nil
}

func (h *HousekeepingJob) Name() string           { return "workflow-kernel-housekeeping" }
func (h *HousekeepingJob) Schedule() string       { return h.cfg.HousekeepingSchedule }
func (h *HousekeepingJob) Timeout() time.Duration { return 5 * time.Minute }

func (h *HousekeepingJob) Run(ctx context.Context) error {
	retention := time.Duration(h.cfg.HousekeepingRetentionDays) * 24 * time.Hour
	limit := h.cfg.HousekeepingBatchSize
	var total int64
	for {
		var n int64
		err := h.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
			store := h.factory.Store(tx)
			deleted, delErr := store.DeleteCompleted(ctx, retention, limit)
			if delErr != nil {
				return fmt.Errorf("workflow: housekeeping: %w", delErr)
			}
			n = deleted
			return nil
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
		h.logger.Info(ctx, "workflow: housekeeping deleted completed runs",
			observability.Int64("count", total),
		)
	}
	return nil
}
