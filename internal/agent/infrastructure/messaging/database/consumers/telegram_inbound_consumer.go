package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type telegramRouter interface {
	RouteTelegram(ctx context.Context, principal appservices.Principal, msg appservices.InboundMessage) appservices.RouteResult
}

type TelegramInboundConsumer struct {
	router      telegramRouter
	o11y        observability.Observability
	decodeFails observability.Counter
}

func NewTelegramInboundConsumer(router telegramRouter, o11y observability.Observability) *TelegramInboundConsumer {
	decodeFails := o11y.Metrics().Counter(
		"agent_telegram_inbound_consumer_decode_failed_total",
		"Total de falhas de decode do consumer de inbound Telegram",
		"1",
	)
	return &TelegramInboundConsumer{
		router:      router,
		o11y:        o11y,
		decodeFails: decodeFails,
	}
}

func (c *TelegramInboundConsumer) Handle(ctx context.Context, event platformevents.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "agent.consumer.telegram_inbound.handle")
	defer span.End()

	env, ok := event.GetPayload().(outbox.Envelope)
	if !ok {
		return fmt.Errorf("agent.consumer.telegram_inbound: tipo de payload inesperado %T", event.GetPayload())
	}

	var p telegramInboundPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.telegram_inbound: deserializar payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.telegram_inbound: parse user_id: %w", err)
	}

	chatID, err := strconv.ParseInt(p.Peer, 10, 64)
	if err != nil {
		c.decodeFails.Add(ctx, 1)
		return fmt.Errorf("agent.consumer.telegram_inbound: parse chat_id: %w", err)
	}

	c.router.RouteTelegram(ctx,
		appservices.Principal{UserID: userID},
		appservices.InboundMessage{
			Text:       p.Text,
			TelegramTo: chatID,
			MessageID:  p.MessageID,
		},
	)
	return nil
}

type telegramInboundPayload struct {
	UserID    string `json:"user_id"`
	Peer      string `json:"peer"`
	Text      string `json:"text"`
	MessageID string `json:"message_id"`
}
