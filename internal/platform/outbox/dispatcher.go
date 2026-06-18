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
	uow        uow.UnitOfWork
	factory    OutboxRepositoryFactory
	registry   Registry
	cfg        configs.OutboxConfig
	logger     observability.Logger
	mu         sync.Mutex
	rng        *rand.Rand
	instanceID string
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

func (d *DispatcherJob) Name() string           { return "outbox-dispatcher" }
func (d *DispatcherJob) Schedule() string       { return "@every " + d.cfg.DispatcherTickInterval.String() }
func (d *DispatcherJob) Timeout() time.Duration { return 5 * time.Minute }

func (d *DispatcherJob) Run(ctx context.Context) error {
	var rows []Row
	err := d.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		storage := d.factory.OutboxRepository(tx)
		claimed, claimErr := storage.ClaimBatch(ctx, d.instanceID, d.cfg.DispatcherBatchSize)
		if claimErr != nil {
			return fmt.Errorf("outbox: dispatcher claim batch: %w", claimErr)
		}
		var rowErrs []error
		for _, row := range claimed {
			if dispErr := d.dispatch(ctx, row, storage); dispErr != nil {
				rowErrs = append(rowErrs, dispErr)
			}
		}
		rows = claimed
		return errors.Join(rowErrs...)
	})
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		d.logger.Info(ctx, "outbox: dispatcher processed batch",
			observability.Int("count", len(rows)),
		)
	}
	return nil
}

func (d *DispatcherJob) dispatch(ctx context.Context, row Row, storage OutboxRepository) error {
	handlers := d.registry.HandlersOf(row.Type)
	if len(handlers) == 0 {
		if err := storage.MarkFailed(ctx, row.ID, "no handlers registered"); err != nil {
			return fmt.Errorf("outbox: mark failed (no handlers): %w", err)
		}
		d.logger.Error(ctx, "outbox: no handlers registered",
			observability.String("event_id", row.ID),
			observability.String("event_type", row.Type),
		)
		return nil
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

	joined := errors.Join(handlerErrs...)
	if joined == nil {
		if err := storage.MarkPublished(ctx, row.ID); err != nil {
			return fmt.Errorf("outbox: mark published: %w", err)
		}
		return nil
	}

	nextAttempts := row.Attempts + 1
	if nextAttempts >= row.MaxAttempts {
		if err := storage.MarkFailed(ctx, row.ID, joined.Error()); err != nil {
			return fmt.Errorf("outbox: mark failed: %w", err)
		}
		return nil
	}

	backoff := d.CalcBackoff(row.Attempts)
	if err := storage.MarkPendingRetry(ctx, row.ID, joined.Error(), time.Now().UTC().Add(backoff)); err != nil {
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
