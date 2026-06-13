package usecases

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type authEventPayload struct {
	EventID    string  `json:"event_id"`
	UserID     *string `json:"user_id"`
	Kind       string  `json:"kind"`
	Source     string  `json:"source"`
	Reason     *string `json:"reason"`
	OccurredAt string  `json:"occurred_at"`
	RequestID  string  `json:"request_id,omitempty"`
	ClientIP   string  `json:"client_ip,omitempty"`
}

func newAuthEventOutbox(eventID, userID, kind, source, reason, requestID, clientIP string, now time.Time) (outbox.Event, error) {
	payload := authEventPayload{
		EventID:    eventID,
		Kind:       kind,
		Source:     source,
		OccurredAt: now.Format(time.RFC3339),
		RequestID:  requestID,
		ClientIP:   clientIP,
	}
	if userID != "" {
		payload.UserID = &userID
	}
	if reason != "" {
		payload.Reason = &reason
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return outbox.Event{}, fmt.Errorf("marshal auth event payload: %w", err)
	}

	aggregateID := eventID
	if userID != "" {
		aggregateID = userID
	}

	return outbox.Event{
		ID:              eventID,
		Type:            "auth." + kind,
		AggregateType:   "auth_event",
		AggregateID:     aggregateID,
		AggregateUserID: userID,
		Payload:         rawPayload,
		OccurredAt:      now,
	}, nil
}

func parseAuthEvent(raw []byte) (entities.AuthEvent, error) {
	var p authEventPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return entities.AuthEvent{}, fmt.Errorf("decode payload: %w", err)
	}

	eventID, err := uuid.Parse(p.EventID)
	if err != nil {
		return entities.AuthEvent{}, fmt.Errorf("parse event_id: %w", err)
	}

	occurredAt, err := time.Parse(time.RFC3339, p.OccurredAt)
	if err != nil {
		return entities.AuthEvent{}, fmt.Errorf("parse occurred_at: %w", err)
	}

	var userID *uuid.UUID
	if p.UserID != nil {
		uid, parseErr := uuid.Parse(*p.UserID)
		if parseErr != nil {
			return entities.AuthEvent{}, fmt.Errorf("parse user_id: %w", parseErr)
		}
		userID = &uid
	}

	var reason *entities.AuthEventReason
	if p.Reason != nil {
		r := entities.AuthEventReason(*p.Reason)
		reason = &r
	}

	return entities.HydrateAuthEvent(
		eventID,
		occurredAt,
		userID,
		entities.AuthEventKind(p.Kind),
		entities.AuthEventSource(p.Source),
		reason,
		p.RequestID,
		p.ClientIP,
	), nil
}
