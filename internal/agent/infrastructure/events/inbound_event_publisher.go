package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const (
	EventTypeWhatsAppInbound = "agent.whatsapp.inbound.v1"
	EventTypeTelegramInbound = "agent.telegram.inbound.v1"
	_inboundAggregateType    = "agent_inbound"
)

type InboundEventPublisher struct {
	publisher outbox.Publisher
	o11y      observability.Observability
}

func NewInboundEventPublisher(publisher outbox.Publisher, o11y observability.Observability) *InboundEventPublisher {
	return &InboundEventPublisher{publisher: publisher, o11y: o11y}
}

type inboundPayload struct {
	UserID    string `json:"user_id"`
	Channel   string `json:"channel"`
	Peer      string `json:"peer"`
	Text      string `json:"text"`
	MessageID string `json:"message_id"`
}

func (p *InboundEventPublisher) PublishWhatsApp(ctx context.Context, userID uuid.UUID, peer, text, messageID string) error {
	return p.publish(ctx, userID, "whatsapp", peer, text, messageID, EventTypeWhatsAppInbound)
}

func (p *InboundEventPublisher) PublishTelegram(ctx context.Context, userID uuid.UUID, chatID int64, text, messageID string) error {
	return p.publish(ctx, userID, "telegram", strconv.FormatInt(chatID, 10), text, messageID, EventTypeTelegramInbound)
}

func (p *InboundEventPublisher) publish(ctx context.Context, userID uuid.UUID, channel, peer, text, messageID, eventType string) error {
	payload := inboundPayload{
		UserID:    userID.String(),
		Channel:   channel,
		Peer:      peer,
		Text:      text,
		MessageID: messageID,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("agent.inbound.events: marshal payload: %w", err)
	}
	evt := outbox.Event{
		ID:              uuid.NewSHA1(uuid.NameSpaceURL, []byte(eventType+":"+messageID)).String(),
		Type:            eventType,
		AggregateType:   _inboundAggregateType,
		AggregateID:     messageID,
		AggregateUserID: userID.String(),
		Payload:         raw,
		OccurredAt:      time.Now().UTC(),
	}
	if err := p.publisher.Publish(ctx, evt); err != nil {
		return fmt.Errorf("agent.inbound.events: publish %s: %w", eventType, err)
	}
	return nil
}
