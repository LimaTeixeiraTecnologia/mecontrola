package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type AuthEventKind string

const (
	AuthEventKindPrincipalEstablished AuthEventKind = "principal_established"
	AuthEventKindFailed               AuthEventKind = "failed"
	AuthEventKindUnknownUser          AuthEventKind = "unknown_user"
)

type AuthEventSource string

const (
	AuthEventSourceWhatsApp AuthEventSource = "whatsapp"
)

type AuthEventReason string

const (
	AuthEventReasonInvalidSignature AuthEventReason = "invalid_signature"
	AuthEventReasonUnknownWaID      AuthEventReason = "unknown_wa_id"
	AuthEventReasonInvalidCountry   AuthEventReason = "invalid_country"
	AuthEventReasonInvalidPayload   AuthEventReason = "invalid_payload"
	AuthEventReasonRateLimited      AuthEventReason = "rate_limited"
	AuthEventReasonDBUnavailable    AuthEventReason = "db_unavailable"
)

var (
	ErrPrincipalEstablishedRequiresUserID = errors.New("principal_established requires non-zero user id")
	ErrAuthFailedRequiresReason           = errors.New("auth_failed requires non-empty reason")
)

type AuthEvent struct {
	id         uuid.UUID
	occurredAt time.Time
	userID     *uuid.UUID
	kind       AuthEventKind
	source     AuthEventSource
	reason     *AuthEventReason
}

func NewPrincipalEstablished(userID uuid.UUID, source AuthEventSource) (AuthEvent, error) {
	if userID == uuid.Nil {
		return AuthEvent{}, ErrPrincipalEstablishedRequiresUserID
	}
	id, err := uuid.NewV7()
	if err != nil {
		return AuthEvent{}, err
	}
	uid := userID
	return AuthEvent{
		id:         id,
		occurredAt: time.Now().UTC(),
		userID:     &uid,
		kind:       AuthEventKindPrincipalEstablished,
		source:     source,
	}, nil
}

func NewUnknownUser(source AuthEventSource) (AuthEvent, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return AuthEvent{}, err
	}
	return AuthEvent{
		id:         id,
		occurredAt: time.Now().UTC(),
		kind:       AuthEventKindUnknownUser,
		source:     source,
	}, nil
}

func NewAuthFailed(reason AuthEventReason, source AuthEventSource, userID *uuid.UUID) (AuthEvent, error) {
	if reason == "" {
		return AuthEvent{}, ErrAuthFailedRequiresReason
	}
	id, err := uuid.NewV7()
	if err != nil {
		return AuthEvent{}, err
	}
	r := reason
	return AuthEvent{
		id:         id,
		occurredAt: time.Now().UTC(),
		userID:     userID,
		kind:       AuthEventKindFailed,
		source:     source,
		reason:     &r,
	}, nil
}

func HydrateAuthEvent(id uuid.UUID, occurredAt time.Time, userID *uuid.UUID, kind AuthEventKind, source AuthEventSource, reason *AuthEventReason) AuthEvent {
	return AuthEvent{
		id:         id,
		occurredAt: occurredAt,
		userID:     userID,
		kind:       kind,
		source:     source,
		reason:     reason,
	}
}

func (e AuthEvent) ID() uuid.UUID            { return e.id }
func (e AuthEvent) OccurredAt() time.Time    { return e.occurredAt }
func (e AuthEvent) UserID() *uuid.UUID       { return e.userID }
func (e AuthEvent) Kind() AuthEventKind      { return e.kind }
func (e AuthEvent) Source() AuthEventSource  { return e.source }
func (e AuthEvent) Reason() *AuthEventReason { return e.reason }
