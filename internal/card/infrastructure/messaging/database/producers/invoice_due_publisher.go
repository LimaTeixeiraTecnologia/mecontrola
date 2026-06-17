package producers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const eventTypeInvoiceDue = "card.invoice_due.v1"

const aggregateTypeCardInvoiceDue = "card.invoice_due"

type invoiceDuePayload struct {
	UserID     string `json:"user_id"`
	CardID     string `json:"card_id"`
	CardName   string `json:"card_name"`
	LimitCents int64  `json:"limit_cents"`
	DueDate    string `json:"due_date"`
	DaysUntil  int    `json:"days_until"`
}

type InvoiceDuePublisher struct {
	outboxFactory outbox.OutboxRepositoryFactory
	cfg           configs.OutboxConfig
	idGen         id.Generator
	o11y          observability.Observability
}

func NewInvoiceDuePublisher(
	outboxFactory outbox.OutboxRepositoryFactory,
	cfg configs.OutboxConfig,
	idGen id.Generator,
	o11y observability.Observability,
) *InvoiceDuePublisher {
	return &InvoiceDuePublisher{
		outboxFactory: outboxFactory,
		cfg:           cfg,
		idGen:         idGen,
		o11y:          o11y,
	}
}

func (p *InvoiceDuePublisher) Publish(ctx context.Context, db database.DBTX, alert services.InvoiceDueAlert, occurredAt time.Time) error {
	payload := invoiceDuePayload{
		UserID:     alert.UserID.String(),
		CardID:     alert.CardID.String(),
		CardName:   alert.CardName,
		LimitCents: alert.LimitCents,
		DueDate:    alert.DueDate.UTC().Format("2006-01-02"),
		DaysUntil:  alert.DaysUntil,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("card/producer: marshal invoice_due payload: %w", err)
	}

	outboxEvt, err := outbox.NewEvent(outbox.EventInput{
		ID:              p.idGen.NewID(),
		Type:            eventTypeInvoiceDue,
		AggregateType:   aggregateTypeCardInvoiceDue,
		AggregateID:     alert.CardID.String(),
		AggregateUserID: alert.UserID.String(),
		Payload:         raw,
		OccurredAt:      occurredAt,
	})
	if err != nil {
		return fmt.Errorf("card/producer: new invoice_due event: %w", err)
	}

	storage := p.outboxFactory.OutboxRepository(db)
	publisher := outbox.NewObservablePostgresPublisher(storage, p.cfg, p.o11y)
	if err := publisher.Publish(ctx, outboxEvt); err != nil {
		return fmt.Errorf("card/producer: publish invoice_due: %w", err)
	}
	return nil
}
