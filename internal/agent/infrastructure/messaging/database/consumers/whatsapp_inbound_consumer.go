package consumers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type whatsAppRouter interface {
	RouteWhatsApp(ctx context.Context, principal appservices.Principal, msg appservices.InboundMessage) appservices.RouteResult
}

type WhatsAppInboundConsumer struct {
	router      whatsAppRouter
	o11y        observability.Observability
	decodeFails observability.Counter
}

func NewWhatsAppInboundConsumer(router whatsAppRouter, o11y observability.Observability) *WhatsAppInboundConsumer {
	decodeFails := o11y.Metrics().Counter(
		"agent_whatsapp_inbound_consumer_decode_failed_total",
		"Total de falhas de decode do consumer de inbound WhatsApp",
		"1",
	)
	return &WhatsAppInboundConsumer{
		router:      router,
		o11y:        o11y,
		decodeFails: decodeFails,
	}
}

func (c *WhatsAppInboundConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "agent.consumer.whatsapp_inbound.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("agent.consumer.whatsapp_inbound: tipo de payload inesperado %T", event.GetPayload())
	}

	var p whatsAppInboundPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.whatsapp_inbound: deserializar payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.whatsapp_inbound: parse user_id: %w", err)
	}

	c.router.RouteWhatsApp(ctx,
		appservices.Principal{UserID: userID},
		appservices.InboundMessage{
			Text:       p.Text,
			WhatsAppTo: p.Peer,
			MessageID:  p.MessageID,
		},
	)
	return nil
}

type whatsAppInboundPayload struct {
	UserID    string `json:"user_id"`
	Peer      string `json:"peer"`
	Text      string `json:"text"`
	MessageID string `json:"message_id"`
}
