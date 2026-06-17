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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const (
	transactionExpenseSource = "transactions"
	directionOutcome         = 2
)

type upsertExpenseUseCase interface {
	Execute(ctx context.Context, in input.UpsertExpenseInput) (output.ExpenseOutput, error)
}

type transactionCreatedPayload struct {
	AggregateID   string    `json:"aggregate_id"`
	UserID        string    `json:"user_id"`
	OccurredAt    time.Time `json:"occurred_at"`
	Direction     int       `json:"direction"`
	AmountCents   int64     `json:"amount_cents"`
	RefMonth      string    `json:"ref_month"`
	SubcategoryID string    `json:"subcategory_id"`
}

type TransactionCreatedConsumer struct {
	upsert      upsertExpenseUseCase
	o11y        observability.Observability
	decodeFails observability.Counter
	skipped     observability.Counter
}

func NewTransactionCreatedConsumer(
	upsert upsertExpenseUseCase,
	o11y observability.Observability,
) *TransactionCreatedConsumer {
	decodeFails := o11y.Metrics().Counter(
		"budgets_transaction_created_consumer_decode_failed_total",
		"Total de falhas de decode do consumer de transações criadas",
		"1",
	)
	skipped := o11y.Metrics().Counter(
		"budgets_transaction_created_consumer_skipped_total",
		"Total de transações ignoradas pelo consumer de orçamento",
		"1",
	)
	return &TransactionCreatedConsumer{
		upsert:      upsert,
		o11y:        o11y,
		decodeFails: decodeFails,
		skipped:     skipped,
	}
}

func (c *TransactionCreatedConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "budgets.consumer.transaction_created.handle")
	defer span.End()

	rawPayload := event.GetPayload()
	env, ok := rawPayload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("budgets.consumer.transaction_created: unexpected payload type %T", rawPayload)
	}

	var p transactionCreatedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.transaction_created: deserializar payload: %w", err)
	}

	if p.Direction != directionOutcome {
		c.skipped.Add(ctx, 1, observability.String("reason", "income"))
		return nil
	}

	if p.SubcategoryID == "" || p.SubcategoryID == uuid.Nil.String() {
		c.skipped.Add(ctx, 1, observability.String("reason", "missing_subcategory"))
		c.o11y.Logger().Warn(ctx, "budgets.consumer.transaction_created.skipped_missing_subcategory",
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
	})
	if err != nil {
		if errors.Is(err, appinterfaces.ErrExpenseTombstoneConflict) {
			c.skipped.Add(ctx, 1, observability.String("reason", "tombstone"))
			return nil
		}
		return fmt.Errorf("budgets.consumer.transaction_created: upsert expense: %w", err)
	}

	return nil
}
