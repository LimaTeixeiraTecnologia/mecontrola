package entities

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrPendingStateTransitionInvalid = errors.New("budgets: transição de estado de evento pendente inválida")

type PendingState uint8

const (
	PendingStatePending PendingState = iota + 1
	PendingStateApplied
	PendingStateFailed
	PendingStateExpired
)

type PendingEvent struct {
	id                    uuid.UUID
	eventID               uuid.UUID
	source                valueobjects.ProducerSource
	userID                uuid.UUID
	externalTransactionID valueobjects.ExternalTransactionID
	expectedVersion       int64
	mutationKind          valueobjects.MutationKind
	payload               []byte
	state                 PendingState
	receivedAt            time.Time
	transitionedAt        *time.Time
	reason                string
}

func NewPendingEvent(
	eventID uuid.UUID,
	source valueobjects.ProducerSource,
	userID uuid.UUID,
	externalTransactionID valueobjects.ExternalTransactionID,
	expectedVersion int64,
	mutationKind valueobjects.MutationKind,
	payload []byte,
	now time.Time,
) PendingEvent {
	return PendingEvent{
		id:                    uuid.New(),
		eventID:               eventID,
		source:                source,
		userID:                userID,
		externalTransactionID: externalTransactionID,
		expectedVersion:       expectedVersion,
		mutationKind:          mutationKind,
		payload:               payload,
		state:                 PendingStatePending,
		receivedAt:            now,
	}
}

func HydratePendingEvent(
	id uuid.UUID,
	eventID uuid.UUID,
	source valueobjects.ProducerSource,
	userID uuid.UUID,
	externalTransactionID valueobjects.ExternalTransactionID,
	expectedVersion int64,
	mutationKind valueobjects.MutationKind,
	payload []byte,
	state PendingState,
	receivedAt time.Time,
	transitionedAt *time.Time,
	reason string,
) PendingEvent {
	return PendingEvent{
		id:                    id,
		eventID:               eventID,
		source:                source,
		userID:                userID,
		externalTransactionID: externalTransactionID,
		expectedVersion:       expectedVersion,
		mutationKind:          mutationKind,
		payload:               payload,
		state:                 state,
		receivedAt:            receivedAt,
		transitionedAt:        transitionedAt,
		reason:                reason,
	}
}

func (p PendingEvent) ID() uuid.UUID                       { return p.id }
func (p PendingEvent) EventID() uuid.UUID                  { return p.eventID }
func (p PendingEvent) Source() valueobjects.ProducerSource { return p.source }
func (p PendingEvent) UserID() uuid.UUID                   { return p.userID }
func (p PendingEvent) ExternalTransactionID() valueobjects.ExternalTransactionID {
	return p.externalTransactionID
}
func (p PendingEvent) ExpectedVersion() int64                  { return p.expectedVersion }
func (p PendingEvent) MutationKind() valueobjects.MutationKind { return p.mutationKind }
func (p PendingEvent) Payload() []byte                         { return p.payload }
func (p PendingEvent) State() PendingState                     { return p.state }
func (p PendingEvent) ReceivedAt() time.Time                   { return p.receivedAt }
func (p PendingEvent) TransitionedAt() *time.Time              { return p.transitionedAt }
func (p PendingEvent) Reason() string                          { return p.reason }

func (p PendingEvent) IsTerminal() bool {
	return p.state == PendingStateApplied || p.state == PendingStateFailed || p.state == PendingStateExpired
}

func (p PendingEvent) IsExpired(ttl time.Duration, now time.Time) bool {
	return now.Sub(p.receivedAt) > ttl
}

func (p *PendingEvent) Transition(to PendingState, reason string, now time.Time) error {
	if p.IsTerminal() {
		return fmt.Errorf("budgets: estado atual %d é terminal: %w", p.state, ErrPendingStateTransitionInvalid)
	}
	if to == PendingStatePending {
		return fmt.Errorf("budgets: não é possível transitar para pending: %w", ErrPendingStateTransitionInvalid)
	}
	p.state = to
	p.reason = reason
	p.transitionedAt = &now
	return nil
}
