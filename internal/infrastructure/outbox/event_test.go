package outbox_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox"
)

type EventSuite struct {
	suite.Suite
	validID   events.EventID
	validType events.EventName
}

func TestEvent(t *testing.T) {
	suite.Run(t, new(EventSuite))
}

func (s *EventSuite) SetupSuite() {
	id, err := events.NewEventID("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	s.Require().NoError(err)
	s.validID = id

	name, err := events.NewEventName("identity.user-created")
	s.Require().NoError(err)
	s.validType = name
}

func (s *EventSuite) TestNewEvent_Valid() {
	now := time.Now().UTC()
	partitionKey := "tenant-1"
	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            s.validID,
		EventType:     s.validType,
		Version:       2,
		AggregateType: "user",
		AggregateID:   "u-123",
		PartitionKey:  &partitionKey,
		Payload:       json.RawMessage(`{"name":"Alice"}`),
		OccurredAt:    now,
	})
	s.Require().NoError(err)
	s.Equal(s.validID, evt.ID())
	s.Equal(s.validType, evt.Type())
	s.Equal(uint16(2), evt.Version())
	s.Equal("user", evt.AggregateType())
	s.Equal("u-123", evt.AggregateID())
	s.Require().NotNil(evt.PartitionKey())
	s.Equal("tenant-1", *evt.PartitionKey())
	s.Equal(json.RawMessage(`{"name":"Alice"}`), evt.Payload())
	s.Equal(now, evt.OccurredAt())
}

func (s *EventSuite) TestNewEvent_Defaults() {
	before := time.Now().UTC()
	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            s.validID,
		EventType:     s.validType,
		AggregateType: "order",
		AggregateID:   "o-1",
		Payload:       json.RawMessage(`{}`),
	})
	after := time.Now().UTC()
	s.Require().NoError(err)
	s.Equal(uint16(1), evt.Version(), "version default deve ser 1")
	s.Truef(!evt.OccurredAt().Before(before) && !evt.OccurredAt().After(after),
		"occurred_at default deve ser aproximadamente now()")
	s.Nil(evt.PartitionKey())
}

func (s *EventSuite) TestNewEvent_HeadersDefaultNotNil() {
	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            s.validID,
		EventType:     s.validType,
		AggregateType: "order",
		AggregateID:   "o-1",
		Payload:       json.RawMessage(`{}`),
	})
	s.Require().NoError(err)
	s.NotNil(evt.Headers())
}

func (s *EventSuite) TestHeaders_ReturnsDefensiveCopy() {
	headers := outbox.Headers{"correlation_id": "original"}
	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            s.validID,
		EventType:     s.validType,
		AggregateType: "order",
		AggregateID:   "o-1",
		Payload:       json.RawMessage(`{}`),
		Headers:       headers,
	})
	s.Require().NoError(err)

	headers["correlation_id"] = "mutated-source"
	got := evt.Headers()
	got["correlation_id"] = "mutated-return"

	finalHeaders := evt.Headers()
	value, ok := finalHeaders.Get("correlation_id")
	s.True(ok)
	s.Equal("original", value)
}

func (s *EventSuite) TestNewEvent_InvalidID() {
	var emptyID events.EventID
	_, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            emptyID,
		EventType:     s.validType,
		AggregateType: "user",
		AggregateID:   "u-1",
		Payload:       json.RawMessage(`{}`),
	})
	s.Require().Error(err)
	s.True(errors.Is(err, outbox.ErrInvalidEvent))
}

func (s *EventSuite) TestNewEvent_InvalidPayload() {
	scenarios := []struct {
		name    string
		payload json.RawMessage
	}{
		{"payload nulo", nil},
		{"payload vazio", json.RawMessage(``)},
		{"payload nao-JSON", json.RawMessage(`not json`)},
		{"payload quebrado", json.RawMessage(`{broken}`)},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			_, err := outbox.NewEvent(outbox.NewEventParams{
				ID:            s.validID,
				EventType:     s.validType,
				AggregateType: "user",
				AggregateID:   "u-1",
				Payload:       sc.payload,
			})
			s.Require().Error(err)
			s.True(errors.Is(err, outbox.ErrInvalidEvent), "deve wrapping ErrInvalidEvent")
		})
	}
}

func (s *EventSuite) TestNewEvent_MissingAggregateType() {
	_, err := outbox.NewEvent(outbox.NewEventParams{
		ID:          s.validID,
		EventType:   s.validType,
		AggregateID: "u-1",
		Payload:     json.RawMessage(`{}`),
	})
	s.Require().Error(err)
	s.True(errors.Is(err, outbox.ErrInvalidEvent))
}

func (s *EventSuite) TestNewEvent_MissingAggregateID() {
	_, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            s.validID,
		EventType:     s.validType,
		AggregateType: "user",
		Payload:       json.RawMessage(`{}`),
	})
	s.Require().Error(err)
	s.True(errors.Is(err, outbox.ErrInvalidEvent))
}

func (s *EventSuite) TestNewEvent_MissingEventType() {
	_, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            s.validID,
		AggregateType: "user",
		AggregateID:   "u-1",
		Payload:       json.RawMessage(`{}`),
	})
	s.Require().Error(err)
	s.True(errors.Is(err, outbox.ErrInvalidEvent))
	s.Contains(err.Error(), "event type obrigatorio")
}
