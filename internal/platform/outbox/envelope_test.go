package outbox_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type EnvelopeSuite struct {
	suite.Suite
}

func TestEnvelope(t *testing.T) {
	suite.Run(t, new(EnvelopeSuite))
}

func (s *EnvelopeSuite) SetupTest() {}

func (s *EnvelopeSuite) TestPack() {
	type args struct {
		row outbox.Row
	}

	occurred := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(outbox.Envelope)
	}{
		{
			name: "deve preservar campos ao empacotar",
			args: args{
				row: outbox.Row{
					Event: outbox.Event{
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
				},
			},
			setup: func() {},
			expect: func(result outbox.Envelope) {
				s.Equal("abc-123", result.ID)
				s.Equal("test.event", result.EventType)
				s.Equal(occurred, result.OccurredAt)
				s.Equal(map[string]string{"trace_id": "t1"}, result.Metadata)
				s.Equal(json.RawMessage(`{"key":"val"}`), result.Payload)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			sut := outbox.Pack
			result := sut(scenario.args.row)

			scenario.expect(result)
		})
	}
}

func (s *EnvelopeSuite) TestUnpack() {
	type args struct {
		envelope outbox.Envelope
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func([]byte, error)
	}{
		{
			name: "deve produzir json valido ao desempacotar",
			args: args{
				envelope: outbox.Envelope{
					ID:         "abc-123",
					EventType:  "test.event",
					OccurredAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
					Metadata:   map[string]string{"k": "v"},
					Payload:    json.RawMessage(`{"x":1}`),
				},
			},
			setup: func() {},
			expect: func(result []byte, err error) {
				s.NoError(err)
				s.True(json.Valid(result))

				var decoded outbox.Envelope
				s.NoError(json.Unmarshal(result, &decoded))
				s.Equal("abc-123", decoded.ID)
				s.Equal("test.event", decoded.EventType)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			sut := outbox.Unpack
			result, err := sut(scenario.args.envelope)

			scenario.expect(result, err)
		})
	}
}
