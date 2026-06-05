package outbox

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type EnvelopeSuite struct {
	suite.Suite
}

func TestEnvelope(t *testing.T) {
	suite.Run(t, new(EnvelopeSuite))
}

func (s *EnvelopeSuite) TestPack_PreservesFields() {
	occurred := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	row := Row{
		Event: Event{
			ID:            "abc-123",
			Type:          "test.event",
			Payload:       []byte(`{"key":"val"}`),
			Metadata:      map[string]string{"trace_id": "t1"},
			OccurredAt:    occurred,
			AggregateType: "T",
			AggregateID:   "1",
		},
		Attempts:    2,
		MaxAttempts: 15,
	}

	env := Pack(row)

	s.Equal("abc-123", env.ID)
	s.Equal("test.event", env.EventType)
	s.Equal(occurred, env.OccurredAt)
	s.Equal(map[string]string{"trace_id": "t1"}, env.Metadata)
	s.Equal(json.RawMessage(`{"key":"val"}`), env.Payload)
}

func (s *EnvelopeSuite) TestUnpack_ProducesValidJSON() {
	env := Envelope{
		ID:         "abc-123",
		EventType:  "test.event",
		OccurredAt: time.Now().UTC(),
		Metadata:   map[string]string{"k": "v"},
		Payload:    json.RawMessage(`{"x":1}`),
	}

	b, err := Unpack(env)

	s.NoError(err)
	s.True(json.Valid(b))

	var decoded Envelope
	s.NoError(json.Unmarshal(b, &decoded))
	s.Equal(env.ID, decoded.ID)
	s.Equal(env.EventType, decoded.EventType)
}
