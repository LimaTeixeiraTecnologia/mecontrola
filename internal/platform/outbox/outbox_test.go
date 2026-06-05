package outbox

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type NewEventSuite struct {
	suite.Suite
}

func TestNewEvent(t *testing.T) {
	suite.Run(t, new(NewEventSuite))
}

func (s *NewEventSuite) TestNewEvent_Success() {
	payload := []byte(`{"key":"value"}`)
	id := uuid.NewString()
	occurred := time.Now().UTC().Truncate(time.Millisecond)

	evt, err := NewEvent(EventInput{
		ID:            id,
		Type:          "test.event",
		AggregateType: "TestAggregate",
		AggregateID:   "agg-1",
		Payload:       payload,
		Metadata:      map[string]string{"k": "v"},
		OccurredAt:    occurred,
	})

	s.NoError(err)
	s.Equal(id, evt.ID)
	s.Equal("test.event", evt.Type)
	s.Equal("TestAggregate", evt.AggregateType)
	s.Equal("agg-1", evt.AggregateID)
	s.Equal(payload, evt.Payload)
	s.Equal(map[string]string{"k": "v"}, evt.Metadata)
	s.Equal(occurred, evt.OccurredAt)
}

func (s *NewEventSuite) TestNewEvent_GeneratesIDWhenEmpty() {
	evt, err := NewEvent(EventInput{
		Type:          "test.event",
		AggregateType: "A",
		AggregateID:   "1",
		Payload:       []byte(`{}`),
	})
	s.NoError(err)
	s.NotEmpty(evt.ID)
	_, parseErr := uuid.Parse(evt.ID)
	s.NoError(parseErr)
}

func (s *NewEventSuite) TestNewEvent_NormalizesOccurredAtToUTC() {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	t := time.Date(2024, 1, 15, 10, 0, 0, 0, loc)

	evt, err := NewEvent(EventInput{
		Type:          "test.event",
		AggregateType: "A",
		AggregateID:   "1",
		Payload:       []byte(`{}`),
		OccurredAt:    t,
	})
	s.NoError(err)
	s.Equal(time.UTC, evt.OccurredAt.Location())
}

func (s *NewEventSuite) TestNewEvent_UsesNowWhenOccurredAtZero() {
	before := time.Now().UTC()
	evt, err := NewEvent(EventInput{
		Type:          "test.event",
		AggregateType: "A",
		AggregateID:   "1",
		Payload:       []byte(`{}`),
	})
	after := time.Now().UTC()

	s.NoError(err)
	s.True(evt.OccurredAt.After(before) || evt.OccurredAt.Equal(before))
	s.True(evt.OccurredAt.Before(after) || evt.OccurredAt.Equal(after))
}

func (s *NewEventSuite) TestNewEvent_DefensiveCopyPayload() {
	payload := []byte(`{"x":1}`)
	evt, err := NewEvent(EventInput{
		Type:          "test.event",
		AggregateType: "A",
		AggregateID:   "1",
		Payload:       payload,
	})
	s.NoError(err)
	payload[1] = 'Z'
	s.Equal(byte('{'), evt.Payload[0])
	s.NotEqual(payload, evt.Payload)
}

func (s *NewEventSuite) TestNewEvent_MetadataNeverNil() {
	evt, err := NewEvent(EventInput{
		Type:          "test.event",
		AggregateType: "A",
		AggregateID:   "1",
		Payload:       []byte(`{}`),
	})
	s.NoError(err)
	s.NotNil(evt.Metadata)
}

type newEventErrorScenario struct {
	name    string
	input   EventInput
	wantErr error
}

func (s *NewEventSuite) TestNewEvent_Errors() {
	scenarios := []newEventErrorScenario{
		{
			name:    "id invalido nao e uuid",
			input:   EventInput{ID: "not-a-uuid", Type: "t", AggregateType: "A", AggregateID: "1", Payload: []byte(`{}`)},
			wantErr: ErrEventIDMissing,
		},
		{
			name:    "type vazio",
			input:   EventInput{Type: "", AggregateType: "A", AggregateID: "1", Payload: []byte(`{}`)},
			wantErr: ErrEventTypeMissing,
		},
		{
			name:    "aggregate type vazio",
			input:   EventInput{Type: "t", AggregateType: "", AggregateID: "1", Payload: []byte(`{}`)},
			wantErr: ErrAggregateTypeMissing,
		},
		{
			name:    "aggregate id vazio",
			input:   EventInput{Type: "t", AggregateType: "A", AggregateID: "", Payload: []byte(`{}`)},
			wantErr: ErrAggregateIDMissing,
		},
		{
			name:    "payload invalido",
			input:   EventInput{Type: "t", AggregateType: "A", AggregateID: "1", Payload: []byte(`not-json`)},
			wantErr: ErrInvalidPayload,
		},
		{
			name:    "payload nao e objeto - null",
			input:   EventInput{Type: "t", AggregateType: "A", AggregateID: "1", Payload: []byte(`null`)},
			wantErr: ErrInvalidPayload,
		},
		{
			name:    "payload nao e objeto - array",
			input:   EventInput{Type: "t", AggregateType: "A", AggregateID: "1", Payload: []byte(`[1,2]`)},
			wantErr: ErrInvalidPayload,
		},
		{
			name:    "payload nao e objeto - numero",
			input:   EventInput{Type: "t", AggregateType: "A", AggregateID: "1", Payload: []byte(`42`)},
			wantErr: ErrInvalidPayload,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			_, err := NewEvent(sc.input)
			s.ErrorIs(err, sc.wantErr)
		})
	}
}
