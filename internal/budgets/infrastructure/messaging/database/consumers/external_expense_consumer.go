package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type ingestExternalExpenseUseCase interface {
	Execute(ctx context.Context, in usecases.IngestExternalExpenseInput) error
}

type externalExpenseEventPayload struct {
	EventID               string    `json:"event_id"`
	Source                string    `json:"source"`
	ExternalTransactionID string    `json:"external_transaction_id"`
	OccurredAt            time.Time `json:"occurred_at"`
	UserID                string    `json:"user_id"`
	Operation             string    `json:"operation"`
	Version               int64     `json:"version"`
	SubcategoryID         string    `json:"subcategory_id"`
	Competence            string    `json:"competence"`
	AmountCents           int64     `json:"amount_cents"`
}

type ExternalExpenseConsumer struct {
	ingest      ingestExternalExpenseUseCase
	o11y        observability.Observability
	decodeFails observability.Counter
}

func NewExternalExpenseConsumer(
	ingest ingestExternalExpenseUseCase,
	o11y observability.Observability,
) *ExternalExpenseConsumer {
	decodeFails := o11y.Metrics().Counter(
		"budgets_external_expense_consumer_decode_failed_total",
		"Total de falhas de decode do consumer de despesas externas",
		"1",
	)
	return &ExternalExpenseConsumer{
		ingest:      ingest,
		o11y:        o11y,
		decodeFails: decodeFails,
	}
}

func (c *ExternalExpenseConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "budgets.consumer.external_expense.handle")
	defer span.End()

	rawPayload := event.GetPayload()
	env, ok := rawPayload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("budgets.consumer.external_expense: unexpected payload type %T", rawPayload)
	}

	var p externalExpenseEventPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("budgets.consumer.external_expense: deserializar payload: %w", err)
	}

	return c.ingest.Execute(ctx, usecases.IngestExternalExpenseInput{
		EventID:               p.EventID,
		Source:                p.Source,
		ExternalTransactionID: p.ExternalTransactionID,
		OccurredAt:            p.OccurredAt,
		UserID:                p.UserID,
		Operation:             p.Operation,
		Version:               p.Version,
		SubcategoryID:         p.SubcategoryID,
		Competence:            p.Competence,
		AmountCents:           p.AmountCents,
	})
}
