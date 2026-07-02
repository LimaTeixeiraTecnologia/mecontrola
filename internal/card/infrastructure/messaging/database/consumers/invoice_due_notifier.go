package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const eventTypeInvoiceDue = "card.invoice_due.v1"

type notifyInvoiceDueUseCase interface {
	Execute(ctx context.Context, in usecases.NotifyInvoiceDueInput) (usecases.NotifyInvoiceDueResult, error)
}

type invoiceDuePayload struct {
	UserID       string `json:"user_id"`
	CardID       string `json:"card_id"`
	CardNickname string `json:"card_nickname"`
	DueDate      string `json:"due_date"`
	DaysUntil    int    `json:"days_until"`
}

type InvoiceDueNotifier struct {
	notify      notifyInvoiceDueUseCase
	o11y        observability.Observability
	decodeFails observability.Counter
}

func NewInvoiceDueNotifier(notify notifyInvoiceDueUseCase, o11y observability.Observability) *InvoiceDueNotifier {
	decodeFails := o11y.Metrics().Counter(
		"card_invoice_due_notifier_decode_failed_total",
		"Total de falhas de decode do consumer invoice_due_notifier",
		"1",
	)
	return &InvoiceDueNotifier{
		notify:      notify,
		o11y:        o11y,
		decodeFails: decodeFails,
	}
}

func (c *InvoiceDueNotifier) Handle(ctx context.Context, event events.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "card.consumer.invoice_due_notifier.handle")
	defer span.End()

	if event.GetEventType() != eventTypeInvoiceDue {
		return fmt.Errorf("card.consumer.invoice_due_notifier: unhandled event type %q", event.GetEventType())
	}

	payload := event.GetPayload()
	env, ok := payload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("card.consumer.invoice_due_notifier: unexpected payload type %T", payload)
	}

	var p invoiceDuePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("card.consumer.invoice_due_notifier: deserializar payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("card.consumer.invoice_due_notifier: user_id invalido: %w", err)
	}
	cardID, err := uuid.Parse(p.CardID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("card.consumer.invoice_due_notifier: card_id invalido: %w", err)
	}
	dueDate, err := time.Parse("2006-01-02", p.DueDate)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("card.consumer.invoice_due_notifier: due_date invalido: %w", err)
	}

	in := usecases.NotifyInvoiceDueInput{
		UserID:       userID,
		CardID:       cardID,
		CardNickname: p.CardNickname,
		DueDate:      dueDate.UTC(),
		DaysUntil:    p.DaysUntil,
	}

	if _, err := c.notify.Execute(ctx, in); err != nil {
		return fmt.Errorf("card.consumer.invoice_due_notifier: notificar: %w", err)
	}
	return nil
}
