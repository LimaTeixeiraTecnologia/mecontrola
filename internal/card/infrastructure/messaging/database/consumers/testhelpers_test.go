package consumers_test

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"

type stubEvent struct {
	eventType string
	payload   outbox.Envelope
}

func (s stubEvent) GetEventType() string { return s.eventType }
func (s stubEvent) GetPayload() any      { return s.payload }
