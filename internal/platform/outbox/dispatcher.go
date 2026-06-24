package outbox

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

type DispatcherJob struct {
	uow             uow.UnitOfWork
	factory         OutboxRepositoryFactory
	registry        Registry
	cfg             configs.OutboxConfig
	logger          observability.Logger
	deadLetterTotal observability.Counter
	mu              sync.Mutex
	rng             *rand.Rand
	instanceID      string
}

func NewDispatcherJob(
	unitOfWork uow.UnitOfWork,
	factory OutboxRepositoryFactory,
	registry Registry,
	cfg configs.OutboxConfig,
	logger observability.Logger,
	rng *rand.Rand,
) *DispatcherJob {
	return &DispatcherJob{
		uow:        unitOfWork,
		factory:    factory,
		registry:   registry,
		cfg:        cfg,
		logger:     logger,
		rng:        rng,
		instanceID: newInstanceID(),
	}
}

func NewObservableDispatcherJob(
	unitOfWork uow.UnitOfWork,
	factory OutboxRepositoryFactory,
	registry Registry,
	cfg configs.OutboxConfig,
	o11y observability.Observability,
	rng *rand.Rand,
) *DispatcherJob {
	job := NewDispatcherJob(unitOfWork, factory, registry, cfg, o11y.Logger(), rng)
	job.deadLetterTotal = o11y.Metrics().Counter(
		"outbox_dead_letter_total",
		"Total de eventos do outbox movidos para dead-letter por esgotamento de retries",
		"1",
	)
	return job
}

func (d *DispatcherJob) Name() string           { return "outbox-dispatcher" }
func (d *DispatcherJob) Schedule() string       { return "@every " + d.cfg.DispatcherTickInterval.String() }
func (d *DispatcherJob) Timeout() time.Duration { return 5 * time.Minute }

var errNoOutboxHandlers = errors.New("no handlers registered")

func (d *DispatcherJob) Run(ctx context.Context) error {
	var rows []Row
	err := d.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		claimed, claimErr := d.factory.OutboxRepository(tx).ClaimBatch(ctx, d.instanceID, d.cfg.DispatcherBatchSize)
		if claimErr != nil {
			return fmt.Errorf("outbox: dispatcher claim batch: %w", claimErr)
		}
		rows = claimed
		return nil
	})
	if err != nil {
		return err
	}

	var rowErrs []error
	for _, row := range rows {
		if procErr := d.processClaimed(ctx, row); procErr != nil {
			rowErrs = append(rowErrs, procErr)
		}
	}
	if len(rows) > 0 {
		d.logger.Info(ctx, "outbox: dispatcher processed batch",
			observability.Int("count", len(rows)),
		)
	}
	return errors.Join(rowErrs...)
}

func (d *DispatcherJob) processClaimed(ctx context.Context, row Row) error {
	handlerErr := d.runHandlers(ctx, row)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return d.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		return d.mark(ctx, d.factory.OutboxRepository(tx), row, handlerErr)
	})
}

func (d *DispatcherJob) runHandlers(ctx context.Context, row Row) error {
	handlers := d.registry.HandlersOf(row.Type)
	if len(handlers) == 0 {
		d.logger.Error(ctx, "outbox: no handlers registered",
			observability.String("event_id", row.ID),
			observability.String("event_type", row.Type),
		)
		return errNoOutboxHandlers
	}

	envelope := Pack(row)
	evt := &envelopeEvent{eventType: row.Type, envelope: envelope}

	var handlerErrs []error
	for _, h := range handlers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ctxH, cancel := context.WithTimeout(ctx, d.cfg.DispatcherHandlerTimeout)
		hErr := h.Handle(ctxH, evt)
		cancel()
		if hErr != nil {
			handlerErrs = append(handlerErrs, hErr)
		}
	}
	return errors.Join(handlerErrs...)
}

func (d *DispatcherJob) mark(ctx context.Context, storage OutboxRepository, row Row, handlerErr error) error {
	if errors.Is(handlerErr, errNoOutboxHandlers) {
		if err := storage.MarkFailed(ctx, row.ID, "no handlers registered"); err != nil {
			return fmt.Errorf("outbox: mark failed (no handlers): %w", err)
		}
		return nil
	}
	if handlerErr == nil {
		if err := storage.MarkPublished(ctx, row.ID); err != nil {
			return fmt.Errorf("outbox: mark published: %w", err)
		}
		return nil
	}

	nextAttempts := row.Attempts + 1
	if nextAttempts >= row.MaxAttempts {
		if err := storage.MarkFailed(ctx, row.ID, handlerErr.Error()); err != nil {
			return fmt.Errorf("outbox: mark failed: %w", err)
		}
		if d.deadLetterTotal != nil {
			d.deadLetterTotal.Add(ctx, 1, observability.String("event_type", row.Type))
		}
		return nil
	}

	backoff := d.CalcBackoff(row.Attempts)
	if err := storage.MarkPendingRetry(ctx, row.ID, handlerErr.Error(), time.Now().UTC().Add(backoff)); err != nil {
		return fmt.Errorf("outbox: mark pending retry: %w", err)
	}
	return nil
}

func (d *DispatcherJob) CalcBackoff(attempts int) time.Duration {
	base := d.cfg.RetryBaseBackoff
	maxB := d.cfg.RetryMaxBackoff

	backoff := base
	for range attempts {
		backoff *= 2
		if backoff > maxB {
			backoff = maxB
			break
		}
	}

	jitter := float64(backoff) * 0.2
	d.mu.Lock()
	delta := d.rng.Float64()*2*jitter - jitter
	d.mu.Unlock()

	result := time.Duration(float64(backoff) + delta)
	if result < 0 {
		return base
	}
	return result
}

type envelopeEvent struct {
	eventType string
	envelope  Envelope
}

func (e *envelopeEvent) GetEventType() string { return e.eventType }
func (e *envelopeEvent) GetPayload() any      { return e.envelope }
