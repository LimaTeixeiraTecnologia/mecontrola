package events_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type SubscriptionBoundEventSuite struct {
	suite.Suite
}

func TestSubscriptionBoundEventSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionBoundEventSuite))
}

func (s *SubscriptionBoundEventSuite) TestPayloadGolden() {
	boundAt := time.Date(2026, 6, 12, 14, 30, 45, 0, time.UTC)
	domainEvt := entities.SubscriptionBound{
		EventID:         "11111111-1111-1111-1111-111111111111",
		TokenID:         "22222222-2222-2222-2222-222222222222",
		UserID:          "user-xyz",
		SubscriptionID:  "sub-001",
		TokenHashPrefix: "deadbeef",
		ActivationPath:  valueobjects.ActivationPathDirect,
		BoundAt:         boundAt,
	}

	envelope, err := events.NewSubscriptionBoundEvent(domainEvt)
	s.Require().NoError(err)

	var payload map[string]any
	s.Require().NoError(json.Unmarshal(envelope.Payload, &payload))

	expected := map[string]any{
		"event_id":          "11111111-1111-1111-1111-111111111111",
		"user_id":           "user-xyz",
		"subscription_id":   "sub-001",
		"token_hash_prefix": "deadbeef",
		"activation_path":   "direct",
		"bound_at":          "2026-06-12T14:30:45Z",
	}
	for k, v := range expected {
		s.Equalf(v, payload[k], "payload key %q", k)
	}
	s.Lenf(payload, len(expected), "payload must contain exactly %d keys; got %d (%v)", len(expected), len(payload), payload)

	s.Equal("11111111-1111-1111-1111-111111111111", envelope.ID)
	s.Equal("onboarding.subscription_bound", envelope.Type)
	s.Equal("onboarding_token", envelope.AggregateType)
	s.Equal("22222222-2222-2222-2222-222222222222", envelope.AggregateID)
	s.Equal(boundAt, envelope.OccurredAt)
}

func (s *SubscriptionBoundEventSuite) TestPayloadByteStability() {
	boundAt := time.Date(2026, 6, 12, 14, 30, 45, 0, time.UTC)
	domainEvt := entities.SubscriptionBound{
		EventID:         "11111111-1111-1111-1111-111111111111",
		TokenID:         "22222222-2222-2222-2222-222222222222",
		UserID:          "user-xyz",
		SubscriptionID:  "sub-001",
		TokenHashPrefix: "deadbeef",
		ActivationPath:  valueobjects.ActivationPathDirect,
		BoundAt:         boundAt,
	}

	a, errA := events.NewSubscriptionBoundEvent(domainEvt)
	b, errB := events.NewSubscriptionBoundEvent(domainEvt)
	s.Require().NoError(errA)
	s.Require().NoError(errB)
	s.Equal(string(a.Payload), string(b.Payload))

	const expectedJSON = `{"event_id":"11111111-1111-1111-1111-111111111111","user_id":"user-xyz","subscription_id":"sub-001","token_hash_prefix":"deadbeef","activation_path":"direct","bound_at":"2026-06-12T14:30:45Z"}`
	s.Equal(expectedJSON, string(a.Payload))
}
