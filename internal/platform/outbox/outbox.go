package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
)

var (
	ErrEventIDMissing         = errors.New("outbox: event id is required and must be a valid uuid")
	ErrEventTypeMissing       = errors.New("outbox: event type is required")
	ErrAggregateTypeMissing   = errors.New("outbox: aggregate type is required")
	ErrAggregateIDMissing     = errors.New("outbox: aggregate id is required")
	ErrInvalidPayload         = errors.New("outbox: payload must be a valid json object")
	ErrOccurredAtZero         = errors.New("outbox: occurred_at must not be zero")
	ErrInvalidAggregateUserID = errors.New("outbox: aggregate_user_id must be a valid uuid")
)

type Event struct {
	ID              string
	Type            string
	AggregateType   string
	AggregateID     string
	AggregateUserID string
	Payload         []byte
	Metadata        map[string]string
	OccurredAt      time.Time
}

type EventInput struct {
	ID              string
	Type            string
	AggregateType   string
	AggregateID     string
	AggregateUserID string
	Payload         []byte
	Metadata        map[string]string
	OccurredAt      time.Time
}

func NewEvent(input EventInput) (Event, error) {
	id := input.ID
	if id == "" {
		id = uuid.NewString()
	} else {
		if _, err := uuid.Parse(id); err != nil {
			return Event{}, ErrEventIDMissing
		}
	}
	if input.Type == "" {
		return Event{}, ErrEventTypeMissing
	}
	if input.AggregateType == "" {
		return Event{}, ErrAggregateTypeMissing
	}
	if input.AggregateID == "" {
		return Event{}, ErrAggregateIDMissing
	}
	if input.AggregateUserID != "" {
		if _, err := uuid.Parse(input.AggregateUserID); err != nil {
			return Event{}, ErrInvalidAggregateUserID
		}
	} else if !isSystemEvent(input.Type) && !isNoUserEvent(input.Type) {
		slog.Warn("outbox.event.missing_aggregate_user_id", "event_type", input.Type)
	}
	if !json.Valid(input.Payload) {
		return Event{}, ErrInvalidPayload
	}
	trimmed := bytes.TrimLeft(input.Payload, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return Event{}, ErrInvalidPayload
	}

	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	} else {
		occurredAt = occurredAt.UTC()
	}

	payload := make([]byte, len(input.Payload))
	copy(payload, input.Payload)

	meta := make(map[string]string, len(input.Metadata))
	maps.Copy(meta, input.Metadata)

	return Event{
		ID:              id,
		Type:            input.Type,
		AggregateType:   input.AggregateType,
		AggregateID:     input.AggregateID,
		AggregateUserID: input.AggregateUserID,
		Payload:         payload,
		Metadata:        meta,
		OccurredAt:      occurredAt,
	}, nil
}

type Row struct {
	Event
	Attempts    int
	MaxAttempts int
}

type Publisher interface {
	Publish(ctx context.Context, evt Event) error
}

type Storage interface {
	Insert(ctx context.Context, evt Event, maxAttempts int) error
	ClaimBatch(ctx context.Context, lockedBy string, batchSize int) ([]Row, error)
	MarkPublished(ctx context.Context, id string) error
	MarkPendingRetry(ctx context.Context, id string, lastErr string, nextAttemptAt time.Time) error
	MarkFailed(ctx context.Context, id string, lastErr string) error
	ResetStuck(ctx context.Context, stuckAfter time.Duration) (int64, error)
	DeletePublishedBatch(ctx context.Context, retention time.Duration, limit int) (int64, error)
}

type Registry interface {
	HandlersOf(eventType string) []events.Handler
}
