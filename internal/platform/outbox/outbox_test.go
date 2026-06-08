package outbox_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type NewEventSuite struct {
	suite.Suite
}

func TestNewEvent(t *testing.T) {
	suite.Run(t, new(NewEventSuite))
}

func (s *NewEventSuite) SetupTest() {}

func (s *NewEventSuite) TestNewEvent() {
	type args struct {
		input outbox.EventInput
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(outbox.Event, error)
	}{
		{
			name: "deve criar evento com sucesso",
			args: args{
				input: outbox.EventInput{
					ID:            uuid.NewString(),
					Type:          "test.event",
					AggregateType: "TestAggregate",
					AggregateID:   "agg-1",
					Payload:       []byte(`{"key":"value"}`),
					Metadata:      map[string]string{"k": "v"},
					OccurredAt:    time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
				},
			},
			setup: func() {},
			expect: func(event outbox.Event, err error) {
				s.NoError(err)
				s.Equal("test.event", event.Type)
				s.Equal("TestAggregate", event.AggregateType)
				s.Equal("agg-1", event.AggregateID)
				s.Equal([]byte(`{"key":"value"}`), event.Payload)
				s.Equal(map[string]string{"k": "v"}, event.Metadata)
			},
		},
		{
			name: "deve gerar id quando vazio",
			args: args{
				input: outbox.EventInput{
					Type:          "test.event",
					AggregateType: "A",
					AggregateID:   "1",
					Payload:       []byte(`{}`),
				},
			},
			setup: func() {},
			expect: func(event outbox.Event, err error) {
				s.NoError(err)
				s.NotEmpty(event.ID)
				_, parseErr := uuid.Parse(event.ID)
				s.NoError(parseErr)
			},
		},
		{
			name: "deve normalizar occurred at para utc",
			args: func() args {
				location, err := time.LoadLocation("America/Sao_Paulo")
				s.Require().NoError(err)
				return args{
					input: outbox.EventInput{
						Type:          "test.event",
						AggregateType: "A",
						AggregateID:   "1",
						Payload:       []byte(`{}`),
						OccurredAt:    time.Date(2024, 1, 15, 10, 0, 0, 0, location),
					},
				}
			}(),
			setup: func() {},
			expect: func(event outbox.Event, err error) {
				s.NoError(err)
				s.Equal(time.UTC, event.OccurredAt.Location())
			},
		},
		{
			name: "deve usar now quando occurred at for zero",
			args: args{
				input: outbox.EventInput{
					Type:          "test.event",
					AggregateType: "A",
					AggregateID:   "1",
					Payload:       []byte(`{}`),
				},
			},
			setup: func() {},
			expect: func(event outbox.Event, err error) {
				s.NoError(err)
				s.False(event.OccurredAt.IsZero())
			},
		},
		{
			name: "deve copiar payload defensivamente",
			args: args{
				input: outbox.EventInput{
					Type:          "test.event",
					AggregateType: "A",
					AggregateID:   "1",
					Payload:       []byte(`{"x":1}`),
				},
			},
			setup: func() {},
			expect: func(event outbox.Event, err error) {
				s.NoError(err)
				payload := []byte(`{"x":1}`)
				payload[1] = 'Z'
				s.NotEqual(payload, event.Payload)
			},
		},
		{
			name: "deve garantir metadata nao nil",
			args: args{
				input: outbox.EventInput{
					Type:          "test.event",
					AggregateType: "A",
					AggregateID:   "1",
					Payload:       []byte(`{}`),
				},
			},
			setup: func() {},
			expect: func(event outbox.Event, err error) {
				s.NoError(err)
				s.NotNil(event.Metadata)
			},
		},
		{
			name: "deve retornar erro para id invalido",
			args: args{
				input: outbox.EventInput{
					ID:            "not-a-uuid",
					Type:          "t",
					AggregateType: "A",
					AggregateID:   "1",
					Payload:       []byte(`{}`),
				},
			},
			setup: func() {},
			expect: func(_ outbox.Event, err error) {
				s.ErrorIs(err, outbox.ErrEventIDMissing)
			},
		},
		{
			name: "deve retornar erro para type vazio",
			args: args{
				input: outbox.EventInput{
					AggregateType: "A",
					AggregateID:   "1",
					Payload:       []byte(`{}`),
				},
			},
			setup: func() {},
			expect: func(_ outbox.Event, err error) {
				s.ErrorIs(err, outbox.ErrEventTypeMissing)
			},
		},
		{
			name: "deve retornar erro para aggregate type vazio",
			args: args{
				input: outbox.EventInput{
					Type:        "t",
					AggregateID: "1",
					Payload:     []byte(`{}`),
				},
			},
			setup: func() {},
			expect: func(_ outbox.Event, err error) {
				s.ErrorIs(err, outbox.ErrAggregateTypeMissing)
			},
		},
		{
			name: "deve retornar erro para aggregate id vazio",
			args: args{
				input: outbox.EventInput{
					Type:          "t",
					AggregateType: "A",
					Payload:       []byte(`{}`),
				},
			},
			setup: func() {},
			expect: func(_ outbox.Event, err error) {
				s.ErrorIs(err, outbox.ErrAggregateIDMissing)
			},
		},
		{
			name: "deve retornar erro para payload invalido",
			args: args{
				input: outbox.EventInput{
					Type:          "t",
					AggregateType: "A",
					AggregateID:   "1",
					Payload:       []byte(`not-json`),
				},
			},
			setup: func() {},
			expect: func(_ outbox.Event, err error) {
				s.ErrorIs(err, outbox.ErrInvalidPayload)
			},
		},
		{
			name: "deve retornar erro para payload nao objeto",
			args: args{
				input: outbox.EventInput{
					Type:          "t",
					AggregateType: "A",
					AggregateID:   "1",
					Payload:       []byte(`null`),
				},
			},
			setup: func() {},
			expect: func(_ outbox.Event, err error) {
				s.ErrorIs(err, outbox.ErrInvalidPayload)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			sut := outbox.NewEvent
			event, err := sut(scenario.args.input)

			scenario.expect(event, err)
		})
	}
}
