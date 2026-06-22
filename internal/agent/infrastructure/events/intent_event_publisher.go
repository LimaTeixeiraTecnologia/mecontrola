package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const (
	eventTypeExecuted = "agent.intent.executed.v1"
	eventTypeRejected = "agent.intent.rejected.v1"
	aggregateType     = "agent_session"
)

type IntentEventPublisher struct {
	publisher outbox.Publisher
	o11y      observability.Observability
}

func NewIntentEventPublisher(publisher outbox.Publisher, o11y observability.Observability) *IntentEventPublisher {
	return &IntentEventPublisher{publisher: publisher, o11y: o11y}
}

type intentEventPayload struct {
	EventID          string `json:"event_id"`
	UserID           string `json:"user_id"`
	Channel          string `json:"channel"`
	Outcome          string `json:"outcome"`
	Module           string `json:"module,omitempty"`
	Action           string `json:"action,omitempty"`
	ProviderUsed     string `json:"provider_used,omitempty"`
	Reason           string `json:"reason,omitempty"`
	ResponseHint     string `json:"response_hint,omitempty"`
	LatencyMS        int64  `json:"latency_ms,omitempty"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TraceID          string `json:"trace_id,omitempty"`
	OccurredAt       string `json:"occurred_at"`
}

func (p *IntentEventPublisher) PublishExecuted(ctx context.Context, ev interfaces.IntentEvent) error {
	return p.publish(ctx, ev, eventTypeExecuted)
}

func (p *IntentEventPublisher) PublishRejected(ctx context.Context, ev interfaces.IntentEvent) error {
	return p.publish(ctx, ev, eventTypeRejected)
}

func (p *IntentEventPublisher) publish(ctx context.Context, ev interfaces.IntentEvent, eventType string) error {
	occurredAt := ev.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	payload := intentEventPayload{
		EventID:          ev.EventID.String(),
		UserID:           ev.UserID.String(),
		Channel:          ev.Channel,
		Outcome:          ev.Outcome,
		Module:           ev.Module,
		Action:           ev.Action,
		ProviderUsed:     ev.ProviderUsed.String(),
		Reason:           ev.Reason,
		ResponseHint:     ev.ResponseHint,
		LatencyMS:        ev.LatencyMS,
		PromptTokens:     ev.PromptTokens,
		CompletionTokens: ev.CompletionTokens,
		TraceID:          ev.TraceID,
		OccurredAt:       occurredAt.Format(time.RFC3339),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("agent.llm.events: marshal payload: %w", err)
	}

	outboxEvent := outbox.Event{
		ID:              ev.EventID.String(),
		Type:            eventType,
		AggregateType:   aggregateType,
		AggregateID:     ev.EventID.String(),
		AggregateUserID: ev.UserID.String(),
		Payload:         raw,
		OccurredAt:      occurredAt,
	}
	if err := p.publisher.Publish(ctx, outboxEvent); err != nil {
		return fmt.Errorf("agent.llm.events: publish %s: %w", eventType, err)
	}
	return nil
}
