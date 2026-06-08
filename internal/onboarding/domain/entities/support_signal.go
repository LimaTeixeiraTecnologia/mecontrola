package entities

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type SupportSignal struct {
	id         string
	kind       valueobjects.SupportSignalKind
	payload    json.RawMessage
	occurredAt time.Time
	resolvedAt time.Time
	resolvedBy string
	notes      string
}

func NewSupportSignal(id string, kind valueobjects.SupportSignalKind, payload json.RawMessage) (SupportSignal, error) {
	if id == "" {
		return SupportSignal{}, fmt.Errorf("onboarding: support signal id is required")
	}
	if !json.Valid(payload) {
		return SupportSignal{}, fmt.Errorf("onboarding: support signal payload must be valid json")
	}
	return SupportSignal{
		id:         id,
		kind:       kind,
		payload:    payload,
		occurredAt: time.Now().UTC(),
	}, nil
}

func HydrateSupportSignal(
	id string,
	kind valueobjects.SupportSignalKind,
	payload json.RawMessage,
	occurredAt time.Time,
	resolvedAt time.Time,
	resolvedBy string,
	notes string,
) SupportSignal {
	return SupportSignal{
		id:         id,
		kind:       kind,
		payload:    payload,
		occurredAt: occurredAt,
		resolvedAt: resolvedAt,
		resolvedBy: resolvedBy,
		notes:      notes,
	}
}

func (s SupportSignal) ID() string                           { return s.id }
func (s SupportSignal) Kind() valueobjects.SupportSignalKind { return s.kind }
func (s SupportSignal) Payload() json.RawMessage             { return s.payload }
func (s SupportSignal) OccurredAt() time.Time                { return s.occurredAt }
func (s SupportSignal) ResolvedAt() time.Time                { return s.resolvedAt }
func (s SupportSignal) ResolvedBy() string                   { return s.resolvedBy }
func (s SupportSignal) Notes() string                        { return s.notes }
func (s SupportSignal) IsResolved() bool                     { return !s.resolvedAt.IsZero() }
