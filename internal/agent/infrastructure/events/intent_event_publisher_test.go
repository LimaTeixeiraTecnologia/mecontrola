package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type capturePublisher struct {
	called bool
	got    outbox.Event
	err    error
}

func (c *capturePublisher) Publish(_ context.Context, ev outbox.Event) error {
	c.called = true
	c.got = ev
	return c.err
}

func buildEvent() interfaces.IntentEvent {
	return interfaces.IntentEvent{
		EventID:          uuid.New(),
		UserID:           uuid.New(),
		Channel:          "whatsapp",
		Outcome:          "routed",
		Module:           "transactions",
		Action:           "create",
		ProviderUsed:     valueobjects.ModelSlugGeminiFlashLite(),
		Reason:           "",
		ResponseHint:     "Anotado.",
		LatencyMS:        850,
		PromptTokens:     720,
		CompletionTokens: 80,
		OccurredAt:       time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
	}
}

func TestIntentEventPublisher_PublishExecuted_EmitsCorrectShape(t *testing.T) {
	pub := &capturePublisher{}
	sut := events.NewIntentEventPublisher(pub, noop.NewProvider())

	ev := buildEvent()
	err := sut.PublishExecuted(context.Background(), ev)
	require.NoError(t, err)
	require.True(t, pub.called)

	assert.Equal(t, "agent.intent.executed.v1", pub.got.Type)
	assert.Equal(t, "agent_session", pub.got.AggregateType)
	assert.Equal(t, ev.EventID.String(), pub.got.ID)
	assert.Equal(t, ev.EventID.String(), pub.got.AggregateID)
	assert.Equal(t, ev.UserID.String(), pub.got.AggregateUserID)
	assert.Equal(t, ev.OccurredAt, pub.got.OccurredAt)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(pub.got.Payload, &payload))
	assert.Equal(t, "transactions", payload["module"])
	assert.Equal(t, "create", payload["action"])
	assert.Equal(t, "whatsapp", payload["channel"])
	assert.Equal(t, "google/gemini-2.5-flash-lite", payload["provider_used"])
	assert.Equal(t, float64(850), payload["latency_ms"])
	assert.Equal(t, float64(720), payload["prompt_tokens"])
	assert.Equal(t, float64(80), payload["completion_tokens"])
}

func TestIntentEventPublisher_PublishRejected_EmitsCorrectType(t *testing.T) {
	pub := &capturePublisher{}
	sut := events.NewIntentEventPublisher(pub, noop.NewProvider())

	ev := buildEvent()
	ev.Outcome = "structured_error"
	ev.Reason = "out_of_scope"
	err := sut.PublishRejected(context.Background(), ev)
	require.NoError(t, err)

	assert.Equal(t, "agent.intent.rejected.v1", pub.got.Type)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(pub.got.Payload, &payload))
	assert.Equal(t, "structured_error", payload["outcome"])
	assert.Equal(t, "out_of_scope", payload["reason"])
}

func TestIntentEventPublisher_PublisherError_Propagates(t *testing.T) {
	pub := &capturePublisher{err: errors.New("outbox unavailable")}
	sut := events.NewIntentEventPublisher(pub, noop.NewProvider())

	err := sut.PublishExecuted(context.Background(), buildEvent())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outbox unavailable")
}

func TestIntentEventPublisher_ZeroOccurredAt_UsesNow(t *testing.T) {
	pub := &capturePublisher{}
	sut := events.NewIntentEventPublisher(pub, noop.NewProvider())

	ev := buildEvent()
	ev.OccurredAt = time.Time{}
	require.NoError(t, sut.PublishExecuted(context.Background(), ev))

	assert.False(t, pub.got.OccurredAt.IsZero(), "OccurredAt zero deve ser substituído por now()")
}

func TestIntentEventPublisher_AggregateUserIDMatchesUserID(t *testing.T) {
	pub := &capturePublisher{}
	sut := events.NewIntentEventPublisher(pub, noop.NewProvider())

	ev := buildEvent()
	require.NoError(t, sut.PublishExecuted(context.Background(), ev))

	assert.Equal(t, ev.UserID.String(), pub.got.AggregateUserID,
		"aggregate_user_id obrigatório para auditoria — não pode ser vazio")
}
