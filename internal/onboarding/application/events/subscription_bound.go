package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
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
	PeerE164        string    `json:"peer_e164"`
	BoundAt         time.Time `json:"bound_at"`
}

func NewSubscriptionBoundEvent(evt entities.SubscriptionBound) (outbox.Event, error) {
	payload := subscriptionBoundPayload{
		EventID:         evt.EventID,
		UserID:          evt.UserID,
		SubscriptionID:  evt.SubscriptionID,
		TokenHashPrefix: evt.TokenHashPrefix,
		ActivationPath:  evt.ActivationPath.String(),
		PeerE164:        evt.PeerE164,
		BoundAt:         evt.BoundAt,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return outbox.Event{}, fmt.Errorf("onboarding/event: marshal subscription bound payload: %w", err)
	}

	envelope, err := outbox.NewEvent(outbox.EventInput{
		ID:              evt.EventID,
		Type:            eventTypeSubscriptionBound,
		AggregateType:   aggregateTypeOnboardingToken,
		AggregateID:     evt.TokenID,
		AggregateUserID: evt.UserID,
		Payload:         raw,
		OccurredAt:      evt.BoundAt,
	})
	if err != nil {
		return outbox.Event{}, fmt.Errorf("onboarding/event: new event: %w", err)
	}

	return envelope, nil
}
