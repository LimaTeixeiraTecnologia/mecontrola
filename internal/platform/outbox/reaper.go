package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

type ReaperJob struct {
	uow     uow.UnitOfWork
	factory OutboxRepositoryFactory
	cfg     configs.OutboxConfig
	logger  observability.Logger
}

func NewReaperJob(
	unitOfWork uow.UnitOfWork,
	factory OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	logger observability.Logger,
) *ReaperJob {
	return &ReaperJob{
		uow:     unitOfWork,
		factory: factory,
		cfg:     cfg,
		logger:  logger,
	}
}

func (r *ReaperJob) Name() string           { return "outbox-reaper" }
func (r *ReaperJob) Schedule() string       { return r.cfg.ReaperInterval }
func (r *ReaperJob) Timeout() time.Duration { return 2 * time.Minute }

func (r *ReaperJob) Run(ctx context.Context) error {
	return r.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		storage := r.factory.OutboxRepository(tx)
		n, resetErr := storage.ResetStuck(ctx, r.cfg.ReaperStuckAfter)
		if resetErr != nil {
			return fmt.Errorf("outbox: reaper: %w", resetErr)
		}
		if n > 0 {
			r.logger.Info(ctx, "outbox: reaper reset stuck events",
				observability.Int64("count", n),
			)
		}
		return nil
	})
}
