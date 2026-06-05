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
)

type OutboxDispatcher struct {
	storage    Storage
	registry   Registry
	cfg        configs.OutboxConfig
	logger     observability.Logger
	mu         sync.Mutex
	rng        *rand.Rand
	instanceID string
}

func NewOutboxDispatcher(
	storage Storage,
	registry Registry,
	cfg configs.OutboxConfig,
	logger observability.Logger,
	rng *rand.Rand,
) *OutboxDispatcher {
	return &OutboxDispatcher{
		storage:    storage,
		registry:   registry,
		cfg:        cfg,
		logger:     logger,
		rng:        rng,
		instanceID: newInstanceID(),
	}
}

func (d *OutboxDispatcher) RunOnce(ctx context.Context) error {
	rows, err := d.storage.ClaimBatch(ctx, d.instanceID, d.cfg.DispatcherBatchSize)
	if err != nil {
		return fmt.Errorf("outbox: dispatcher claim batch: %w", err)
	}

	var rowErrs []error
	for _, row := range rows {
		if err := d.dispatch(ctx, row); err != nil {
			rowErrs = append(rowErrs, err)
		}
	}
	return errors.Join(rowErrs...)
}

func (d *OutboxDispatcher) dispatch(ctx context.Context, row Row) error {
	handlers := d.registry.HandlersOf(row.Type)
	if len(handlers) == 0 {
		if err := d.storage.MarkFailed(ctx, row.ID, "no handlers registered"); err != nil {
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
		if err := d.storage.MarkPublished(ctx, row.ID); err != nil {
			return fmt.Errorf("outbox: mark published: %w", err)
		}
		return nil
	}

	nextAttempts := row.Attempts + 1
	if nextAttempts >= row.MaxAttempts {
		if err := d.storage.MarkFailed(ctx, row.ID, joined.Error()); err != nil {
			return fmt.Errorf("outbox: mark failed: %w", err)
		}
		return nil
	}

	backoff := d.CalcBackoff(row.Attempts)
	if err := d.storage.MarkPendingRetry(ctx, row.ID, joined.Error(), time.Now().UTC().Add(backoff)); err != nil {
		return fmt.Errorf("outbox: mark pending retry: %w", err)
	}
	return nil
}

func (d *OutboxDispatcher) CalcBackoff(attempts int) time.Duration {
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
