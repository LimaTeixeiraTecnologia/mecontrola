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
	AuthEventSourceGateway  AuthEventSource = "gateway"
)

type AuthEventReason string

const (
	AuthEventReasonInvalidSignature        AuthEventReason = "invalid_signature"
	AuthEventReasonUnknownWaID             AuthEventReason = "unknown_wa_id"
	AuthEventReasonInvalidCountry          AuthEventReason = "invalid_country"
	AuthEventReasonInvalidPayload          AuthEventReason = "invalid_payload"
	AuthEventReasonRateLimited             AuthEventReason = "rate_limited"
	AuthEventReasonDBUnavailable           AuthEventReason = "db_unavailable"
	AuthEventReasonGatewayMissingHeader    AuthEventReason = "gateway_missing_header"
	AuthEventReasonGatewayInvalidTimestamp AuthEventReason = "gateway_invalid_timestamp"
	AuthEventReasonGatewayStaleTimestamp   AuthEventReason = "gateway_stale_timestamp"
	AuthEventReasonGatewayInvalidSignature AuthEventReason = "gateway_invalid_signature"
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
	requestID  string
	clientIP   string
}

func NewPrincipalEstablished(userID uuid.UUID, source AuthEventSource, requestID, clientIP string) (AuthEvent, error) {
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
		requestID:  requestID,
		clientIP:   clientIP,
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

func NewAuthFailed(reason AuthEventReason, source AuthEventSource, userID *uuid.UUID, requestID, clientIP string) (AuthEvent, error) {
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
		requestID:  requestID,
		clientIP:   clientIP,
	}, nil
}

func HydrateAuthEvent(id uuid.UUID, occurredAt time.Time, userID *uuid.UUID, kind AuthEventKind, source AuthEventSource, reason *AuthEventReason, requestID, clientIP string) AuthEvent {
	return AuthEvent{
		id:         id,
		occurredAt: occurredAt,
		userID:     userID,
		kind:       kind,
		source:     source,
		reason:     reason,
		requestID:  requestID,
		clientIP:   clientIP,
	}
}

func (e AuthEvent) ID() uuid.UUID            { return e.id }
func (e AuthEvent) OccurredAt() time.Time    { return e.occurredAt }
func (e AuthEvent) UserID() *uuid.UUID       { return e.userID }
func (e AuthEvent) Kind() AuthEventKind      { return e.kind }
func (e AuthEvent) Source() AuthEventSource  { return e.source }
func (e AuthEvent) Reason() *AuthEventReason { return e.reason }
func (e AuthEvent) RequestID() string        { return e.requestID }
func (e AuthEvent) ClientIP() string         { return e.clientIP }
