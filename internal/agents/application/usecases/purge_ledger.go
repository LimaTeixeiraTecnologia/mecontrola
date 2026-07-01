package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

const (
	purgeLedgerDefaultRetentionDays = 30
	purgeLedgerDefaultBatchSize     = 10_000
)

type writeLedgerPurger interface {
	DeleteBefore(ctx context.Context, before time.Time, batchSize int) (int64, error)
}

type PurgeLedger struct {
	ledger        writeLedgerPurger
	retentionDays int
	batchSize     int
	o11y          observability.Observability
	deletedTotal  observability.Counter
	durationHist  observability.Histogram
}

func NewPurgeLedger(
	ledger writeLedgerPurger,
	retentionDays int,
	batchSize int,
	o11y observability.Observability,
) *PurgeLedger {
	deletedTotal := o11y.Metrics().Counter(
		"agents_write_ledger_purged_total",
		"Total de entradas do ledger de idempotência removidas pelo housekeeping",
		"1",
	)
	durationHist := o11y.Metrics().Histogram(
		"agents_write_ledger_purge_duration_seconds",
		"Duração de cada execução do housekeeping do ledger de idempotência",
		"s",
	)
	return &PurgeLedger{
		ledger:        ledger,
		retentionDays: retentionDays,
		batchSize:     batchSize,
		o11y:          o11y,
		deletedTotal:  deletedTotal,
		durationHist:  durationHist,
	}
}

func (uc *PurgeLedger) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agents.usecase.purge_ledger")
	defer span.End()

	start := time.Now()

	days := uc.retentionDays
	if days <= 0 {
		days = purgeLedgerDefaultRetentionDays
	}

	batch := uc.batchSize
	if batch <= 0 {
		batch = purgeLedgerDefaultBatchSize
	}

	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	var total int64
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("agents.usecase.purge_ledger: context cancelled: %w", ctx.Err())
		default:
		}

		n, err := uc.ledger.DeleteBefore(ctx, cutoff, batch)
		if err != nil {
			span.RecordError(err)
			uc.o11y.Logger().Error(ctx, "agents.usecase.purge_ledger: delete_before failed",
				observability.Error(err),
			)
			return fmt.Errorf("agents.usecase.purge_ledger: delete_before: %w", err)
		}

		total += n
		if n == 0 {
			break
		}
	}

	elapsed := time.Since(start).Seconds()
	uc.durationHist.Record(ctx, elapsed)

	if total > 0 {
		uc.deletedTotal.Add(ctx, total)
		uc.o11y.Logger().Info(ctx, "agents.usecase.purge_ledger: deleted",
			observability.Int64("count", total),
			observability.Int("retention_days", days),
		)
	}

	return nil
}
