package usecases

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

func newEventID(idGen id.Generator) uuid.UUID {
	parsed, err := uuid.Parse(idGen.NewID())
	if err != nil {
		return uuid.New()
	}
	return parsed
}

func buildOutboxEvent(userID uuid.UUID, evt entities.OnboardingDomainEvent, now time.Time) (outbox.Event, error) {
	payload, err := json.Marshal(evt)
	if err != nil {
		return outbox.Event{}, fmt.Errorf("onboarding: marshal event: %w", err)
	}
	eventID := extractEventID(evt)
	return outbox.NewEvent(outbox.EventInput{
		ID:              eventID.String(),
		Type:            evt.EventType(),
		AggregateType:   "onboarding_session",
		AggregateID:     userID.String(),
		AggregateUserID: userID.String(),
		Payload:         payload,
		OccurredAt:      now,
	})
}

func extractEventID(evt entities.OnboardingDomainEvent) uuid.UUID {
	switch e := evt.(type) {
	case entities.IncomeRegistered:
		return e.EventID
	case entities.CardRegistered:
		return e.EventID
	case entities.SplitsCalculated:
		return e.EventID
	case entities.OnboardingCompleted:
		return e.EventID
	default:
		return uuid.New()
	}
}
