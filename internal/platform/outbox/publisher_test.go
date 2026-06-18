package outbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	dbmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/mocks"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type PublisherSuite struct {
	suite.Suite
}

func TestPublisher(t *testing.T) {
	suite.Run(t, new(PublisherSuite))
}

func (s *PublisherSuite) SetupTest() {}

func (s *PublisherSuite) newValidEvent() outbox.Event {
	event, err := outbox.NewEvent(outbox.EventInput{
		Type:          "test.event",
		AggregateType: "TestAggregate",
		AggregateID:   "agg-1",
		Payload:       []byte(`{"x":1}`),
	})
	s.Require().NoError(err)
	return event
}

func (s *PublisherSuite) TestPublish() {
	type args struct {
		event outbox.Event
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(outbox.Event) (outbox.Publisher, context.Context, *outboxmocks.Storage)
		expect func(error)
	}{
		{
			name: "deve publicar evento com sucesso",
			args: args{event: s.newValidEvent()},
			setup: func(event outbox.Event) (outbox.Publisher, context.Context, *outboxmocks.Storage) {
				storage := outboxmocks.NewStorage(s.T())
				dbtx := dbmocks.NewMockDBTX(s.T())
				ctx := database.WithTx(context.Background(), dbtx)
				storage.EXPECT().Insert(ctx, event, 15).Return(nil).Once()
				return outbox.NewPostgresPublisher(storage, configs.OutboxConfig{RetryMaxAttempts: 15}), ctx, storage
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve retornar erro para id invalido",
			args: func() args {
				event := s.newValidEvent()
				event.ID = "not-a-uuid"
				return args{event: event}
			}(),
			setup: func(outbox.Event) (outbox.Publisher, context.Context, *outboxmocks.Storage) {
				storage := outboxmocks.NewStorage(s.T())
				dbtx := dbmocks.NewMockDBTX(s.T())
				ctx := database.WithTx(context.Background(), dbtx)
				return outbox.NewPostgresPublisher(storage, configs.OutboxConfig{RetryMaxAttempts: 15}), ctx, storage
			},
			expect: func(err error) { s.ErrorIs(err, outbox.ErrEventIDMissing) },
		},
		{
			name: "deve retornar erro para type vazio",
			args: func() args {
				event := s.newValidEvent()
				event.Type = ""
				return args{event: event}
			}(),
			setup: func(outbox.Event) (outbox.Publisher, context.Context, *outboxmocks.Storage) {
				storage := outboxmocks.NewStorage(s.T())
				dbtx := dbmocks.NewMockDBTX(s.T())
				ctx := database.WithTx(context.Background(), dbtx)
				return outbox.NewPostgresPublisher(storage, configs.OutboxConfig{RetryMaxAttempts: 15}), ctx, storage
			},
			expect: func(err error) { s.ErrorIs(err, outbox.ErrEventTypeMissing) },
		},
		{
			name: "deve retornar erro para payload invalido",
			args: func() args {
				event := s.newValidEvent()
				event.Payload = []byte(`not-json`)
				return args{event: event}
			}(),
			setup: func(outbox.Event) (outbox.Publisher, context.Context, *outboxmocks.Storage) {
				storage := outboxmocks.NewStorage(s.T())
				dbtx := dbmocks.NewMockDBTX(s.T())
				ctx := database.WithTx(context.Background(), dbtx)
				return outbox.NewPostgresPublisher(storage, configs.OutboxConfig{RetryMaxAttempts: 15}), ctx, storage
			},
			expect: func(err error) { s.ErrorIs(err, outbox.ErrInvalidPayload) },
		},
		{
			name: "deve retornar erro para payload nao objeto",
			args: func() args {
				event := s.newValidEvent()
				event.Payload = []byte(`null`)
				return args{event: event}
			}(),
			setup: func(outbox.Event) (outbox.Publisher, context.Context, *outboxmocks.Storage) {
				storage := outboxmocks.NewStorage(s.T())
				dbtx := dbmocks.NewMockDBTX(s.T())
				ctx := database.WithTx(context.Background(), dbtx)
				return outbox.NewPostgresPublisher(storage, configs.OutboxConfig{RetryMaxAttempts: 15}), ctx, storage
			},
			expect: func(err error) { s.ErrorIs(err, outbox.ErrInvalidPayload) },
		},
		{
			name: "deve retornar erro para occurred at zero",
			args: func() args {
				event := s.newValidEvent()
				event.OccurredAt = time.Time{}
				return args{event: event}
			}(),
			setup: func(outbox.Event) (outbox.Publisher, context.Context, *outboxmocks.Storage) {
				storage := outboxmocks.NewStorage(s.T())
				dbtx := dbmocks.NewMockDBTX(s.T())
				ctx := database.WithTx(context.Background(), dbtx)
				return outbox.NewPostgresPublisher(storage, configs.OutboxConfig{RetryMaxAttempts: 15}), ctx, storage
			},
			expect: func(err error) { s.ErrorIs(err, outbox.ErrOccurredAtZero) },
		},
		{
			name: "deve ser idempotente para conflitos por id",
			args: args{event: s.newValidEvent()},
			setup: func(event outbox.Event) (outbox.Publisher, context.Context, *outboxmocks.Storage) {
				storage := outboxmocks.NewStorage(s.T())
				dbtx := dbmocks.NewMockDBTX(s.T())
				ctx := database.WithTx(context.Background(), dbtx)
				storage.EXPECT().Insert(ctx, event, 15).Return(nil).Twice()
				return outbox.NewPostgresPublisher(storage, configs.OutboxConfig{RetryMaxAttempts: 15}), ctx, storage
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve propagar erro do storage",
			args: args{event: s.newValidEvent()},
			setup: func(event outbox.Event) (outbox.Publisher, context.Context, *outboxmocks.Storage) {
				storage := outboxmocks.NewStorage(s.T())
				dbtx := dbmocks.NewMockDBTX(s.T())
				ctx := database.WithTx(context.Background(), dbtx)
				storage.EXPECT().Insert(ctx, event, 15).Return(errors.New("db failure")).Once()
				return outbox.NewPostgresPublisher(storage, configs.OutboxConfig{RetryMaxAttempts: 15}), ctx, storage
			},
			expect: func(err error) { s.Error(err) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			publisher, ctx, _ := scenario.setup(scenario.args.event)
			err := publisher.Publish(ctx, scenario.args.event)
			if scenario.name == "deve ser idempotente para conflitos por id" {
				s.NoError(err)
				err = publisher.Publish(ctx, scenario.args.event)
			}
			scenario.expect(err)
		})
	}
}
