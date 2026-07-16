package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type transactionUpdatedPayload struct {
	AggregateID   string    `json:"aggregate_id"`
	UserID        string    `json:"user_id"`
	OccurredAt    time.Time `json:"occurred_at"`
	Direction     int       `json:"direction"`
	AmountCents   int64     `json:"amount_cents"`
	RefMonth      string    `json:"ref_month"`
	SubcategoryID string    `json:"subcategory_id"`
}

type TransactionUpdatedConsumer struct {
	upsert      upsertExpenseUseCase
	o11y        observability.Observability
	decodeFails observability.Counter
	skipped     observability.Counter
}

func NewTransactionUpdatedConsumer(
	upsert upsertExpenseUseCase,
	o11y observability.Observability,
) *TransactionUpdatedConsumer {
	decodeFails := o11y.Metrics().Counter(
		"budgets_transaction_updated_consumer_decode_failed_total",
		"Total de falhas de decode do consumer de transações editadas",
		"1",
	)
	skipped := o11y.Metrics().Counter(
		"budgets_transaction_updated_consumer_skipped_total",
		"Total de transações editadas ignoradas pelo consumer de orçamento",
		"1",
	)
	return &TransactionUpdatedConsumer{
		upsert:      upsert,
		o11y:        o11y,
		decodeFails: decodeFails,
		skipped:     skipped,
	}
}

func (c *TransactionUpdatedConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "budgets.consumer.transaction_updated.handle")
	defer span.End()

	rawPayload := event.GetPayload()
	env, ok := rawPayload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("budgets.consumer.transaction_updated: unexpected payload type %T", rawPayload)
	}

	var p transactionUpdatedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.transaction_updated: deserializar payload: %w", err)
	}

	if p.Direction != directionOutcome {
		c.skipped.Add(ctx, 1, observability.String("reason", "income"))
		return nil
	}

	if p.SubcategoryID == "" || p.SubcategoryID == uuid.Nil.String() {
		c.skipped.Add(ctx, 1, observability.String("reason", "missing_subcategory"))
		c.o11y.Logger().Warn(ctx, "budgets.consumer.transaction_updated.skipped_missing_subcategory",
			observability.String("aggregate_id", p.AggregateID),
			observability.String("ref_month", p.RefMonth),
		)
		return nil
	}

	_, err := c.upsert.Execute(ctx, input.UpsertExpenseInput{
		UserID:                p.UserID,
		Source:                transactionExpenseSource,
		ExternalTransactionID: p.AggregateID,
		SubcategoryID:         p.SubcategoryID,
		Competence:            p.RefMonth,
		AmountCents:           p.AmountCents,
		OccurredAt:            p.OccurredAt,
		Reconcile:             true,
	})
	if err != nil {
		if errors.Is(err, appinterfaces.ErrExpenseTombstoneConflict) {
			c.skipped.Add(ctx, 1, observability.String("reason", "tombstone"))
			return nil
		}
		return fmt.Errorf("budgets.consumer.transaction_updated: upsert expense: %w", err)
	}

	return nil
}
