package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type deleteExpenseByExternalID interface {
	ExecuteByExternalID(ctx context.Context, userID, source, externalTransactionID string) error
}

type transactionDeletedPayload struct {
	AggregateID string    `json:"aggregate_id"`
	UserID      string    `json:"user_id"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type TransactionDeletedConsumer struct {
	deleteExpense deleteExpenseByExternalID
	o11y          observability.Observability
	decodeFails   observability.Counter
	notFound      observability.Counter
}

func NewTransactionDeletedConsumer(
	deleteExpense deleteExpenseByExternalID,
	o11y observability.Observability,
) *TransactionDeletedConsumer {
	decodeFails := o11y.Metrics().Counter(
		"budgets_transaction_deleted_consumer_decode_failed_total",
		"Total de falhas de decode do consumer de transações excluídas",
		"1",
	)
	notFound := o11y.Metrics().Counter(
		"budgets_transaction_deleted_consumer_not_found_total",
		"Total de transações excluídas sem expense correspondente no budget",
		"1",
	)
	return &TransactionDeletedConsumer{
		deleteExpense: deleteExpense,
		o11y:          o11y,
		decodeFails:   decodeFails,
		notFound:      notFound,
	}
}

func (c *TransactionDeletedConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "budgets.consumer.transaction_deleted.handle")
	defer span.End()

	rawPayload := event.GetPayload()
	env, ok := rawPayload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("budgets.consumer.transaction_deleted: unexpected payload type %T", rawPayload)
	}

	var p transactionDeletedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.transaction_deleted: deserializar payload: %w", err)
	}

	if _, err := uuid.Parse(p.AggregateID); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.transaction_deleted: aggregate_id inválido %q: %w", p.AggregateID, err)
	}

	execErr := c.deleteExpense.ExecuteByExternalID(ctx, p.UserID, transactionExpenseSource, p.AggregateID)
	if execErr != nil {
		span.RecordError(execErr)
		return fmt.Errorf("budgets.consumer.transaction_deleted: delete expense: %w", execErr)
	}

	return nil
}
