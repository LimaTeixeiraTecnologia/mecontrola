package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const eventTypeSubscriptionBound = "onboarding.subscription_bound"
const aggregateTypeOnboardingToken = "onboarding_token"

type subscriptionBoundPayload struct {
	EventID         string    `json:"event_id"`
	UserID          string    `json:"user_id"`
	SubscriptionID  string    `json:"subscription_id"`
	TokenHashPrefix string    `json:"token_hash_prefix"`
	ActivationPath  string    `json:"activation_path"`
	BoundAt         time.Time `json:"bound_at"`
}

func NewSubscriptionBoundEvent(
	eventID string,
	userID string,
	token entities.MagicToken,
	path valueobjects.ActivationPath,
	boundAt time.Time,
) (outbox.Event, error) {
	prefix := ""
	if len(token.TokenHash()) > 0 {
		h := fmt.Sprintf("%x", token.TokenHash())
		if len(h) > 8 {
			prefix = h[:8]
		} else {
			prefix = h
		}
	}

	payload := subscriptionBoundPayload{
		EventID:         eventID,
		UserID:          userID,
		SubscriptionID:  token.SubscriptionID(),
		TokenHashPrefix: prefix,
		ActivationPath:  path.String(),
		BoundAt:         boundAt,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return outbox.Event{}, fmt.Errorf("onboarding/event: marshal subscription bound payload: %w", err)
	}

	evt, err := outbox.NewEvent(outbox.EventInput{
		ID:            eventID,
		Type:          eventTypeSubscriptionBound,
		AggregateType: aggregateTypeOnboardingToken,
		AggregateID:   token.ID(),
		Payload:       raw,
		OccurredAt:    boundAt,
	})
	if err != nil {
		return outbox.Event{}, fmt.Errorf("onboarding/event: new event: %w", err)
	}

	return evt, nil
}
