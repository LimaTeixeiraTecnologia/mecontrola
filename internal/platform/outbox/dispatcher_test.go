package outbox_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	eventsmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type noopLogger struct{}

func (noopLogger) Debug(context.Context, string, ...observability.Field) {}
func (noopLogger) Info(context.Context, string, ...observability.Field)  {}
func (noopLogger) Warn(context.Context, string, ...observability.Field)  {}
func (noopLogger) Error(context.Context, string, ...observability.Field) {}
func (noopLogger) With(...observability.Field) observability.Logger      { return noopLogger{} }

type unitOfWorkRows struct {
	err error
}

func (f *unitOfWorkRows) DBTX() database.DBTX { return nil }

func (f *unitOfWorkRows) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	if f.err != nil {
		return f.err
	}
	return fn(ctx, nil)
}

type unitOfWorkVoid struct{}

func (f *unitOfWorkVoid) DBTX() database.DBTX { return nil }

func (f *unitOfWorkVoid) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

type DispatcherSuite struct {
	suite.Suite
	cfg configs.OutboxConfig
}

func TestDispatcher(t *testing.T) {
	suite.Run(t, new(DispatcherSuite))
}

func (s *DispatcherSuite) SetupTest() {
	s.cfg = configs.OutboxConfig{
		DispatcherBatchSize:      3,
		DispatcherHandlerTimeout: 5 * time.Second,
		RetryMaxAttempts:         15,
		RetryBaseBackoff:         2 * time.Second,
		RetryMaxBackoff:          5 * time.Minute,
	}
}

func (s *DispatcherSuite) makeRow(attempts, maxAttempts int) outbox.Row {
	event, err := outbox.NewEvent(outbox.EventInput{
		Type:          "test.event",
		AggregateType: "A",
		AggregateID:   "1",
		Payload:       []byte(`{}`),
	})
	s.Require().NoError(err)
	return outbox.Row{Event: event, Attempts: attempts, MaxAttempts: maxAttempts}
}

func (s *DispatcherSuite) TestRun() {
	type args struct {
		ctx context.Context
		row outbox.Row
	}

	type dependencies struct {
		storage  *outboxmocks.Storage
		factory  *outboxmocks.OutboxRepositoryFactory
		registry *outboxmocks.Registry
	}

	handlerErr := errors.New("handler failed")

	scenarios := []struct {
		name   string
		args   args
		setup  func(args) dependencies
		expect func(error)
	}{
		{
			name: "deve publicar evento com um handler",
			args: args{ctx: context.Background(), row: s.makeRow(0, 15)},
			setup: func(input args) dependencies {
				storage := outboxmocks.NewStorage(s.T())
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				registry := outboxmocks.NewRegistry(s.T())
				handler := eventsmocks.NewHandler(s.T())

				handler.EXPECT().Handle(mock.Anything, mock.Anything).Return(nil).Once()
				registry.EXPECT().HandlersOf("test.event").Return([]events.Handler{handler}).Once()
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{input.row}, nil).Once()
				storage.EXPECT().MarkPublished(mock.Anything, input.row.ID).Return(nil).Once()

				return dependencies{storage: storage, factory: factory, registry: registry}
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve publicar evento com multiplos handlers",
			args: args{ctx: context.Background(), row: s.makeRow(0, 15)},
			setup: func(input args) dependencies {
				storage := outboxmocks.NewStorage(s.T())
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				registry := outboxmocks.NewRegistry(s.T())
				handlers := make([]events.Handler, 0, 3)
				for range 3 {
					handler := eventsmocks.NewHandler(s.T())
					handler.EXPECT().Handle(mock.Anything, mock.Anything).Return(nil).Once()
					handlers = append(handlers, handler)
				}

				registry.EXPECT().HandlersOf("test.event").Return(handlers).Once()
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{input.row}, nil).Once()
				storage.EXPECT().MarkPublished(mock.Anything, input.row.ID).Return(nil).Once()

				return dependencies{storage: storage, factory: factory, registry: registry}
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve reagendar retry em falha parcial",
			args: args{ctx: context.Background(), row: s.makeRow(0, 15)},
			setup: func(input args) dependencies {
				storage := outboxmocks.NewStorage(s.T())
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				registry := outboxmocks.NewRegistry(s.T())

				successHandler := eventsmocks.NewHandler(s.T())
				successHandler.EXPECT().Handle(mock.Anything, mock.Anything).Return(nil).Once()
				failingHandler := eventsmocks.NewHandler(s.T())
				failingHandler.EXPECT().Handle(mock.Anything, mock.Anything).Return(handlerErr).Once()

				registry.EXPECT().HandlersOf("test.event").Return([]events.Handler{successHandler, failingHandler}).Once()
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{input.row}, nil).Once()
				storage.EXPECT().MarkPendingRetry(mock.Anything, input.row.ID, handlerErr.Error(), mock.AnythingOfType("time.Time")).Return(nil).Once()

				return dependencies{storage: storage, factory: factory, registry: registry}
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve marcar evento como falho ao atingir max attempts",
			args: args{ctx: context.Background(), row: s.makeRow(14, 15)},
			setup: func(input args) dependencies {
				storage := outboxmocks.NewStorage(s.T())
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				registry := outboxmocks.NewRegistry(s.T())
				handler := eventsmocks.NewHandler(s.T())

				handler.EXPECT().Handle(mock.Anything, mock.Anything).Return(handlerErr).Once()
				registry.EXPECT().HandlersOf("test.event").Return([]events.Handler{handler}).Once()
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{input.row}, nil).Once()
				storage.EXPECT().MarkFailed(mock.Anything, input.row.ID, handlerErr.Error()).Return(nil).Once()

				return dependencies{storage: storage, factory: factory, registry: registry}
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve marcar falha quando nao houver handlers",
			args: args{ctx: context.Background(), row: s.makeRow(0, 15)},
			setup: func(input args) dependencies {
				storage := outboxmocks.NewStorage(s.T())
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				registry := outboxmocks.NewRegistry(s.T())

				registry.EXPECT().HandlersOf("test.event").Return(nil).Once()
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{input.row}, nil).Once()
				storage.EXPECT().MarkFailed(mock.Anything, input.row.ID, "no handlers registered").Return(nil).Once()

				return dependencies{storage: storage, factory: factory, registry: registry}
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve abortar quando contexto estiver cancelado",
			args: func() args {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return args{ctx: ctx, row: s.makeRow(0, 15)}
			}(),
			setup: func(input args) dependencies {
				storage := outboxmocks.NewStorage(s.T())
				factory := outboxmocks.NewOutboxRepositoryFactory(s.T())
				registry := outboxmocks.NewRegistry(s.T())
				handler := eventsmocks.NewHandler(s.T())

				registry.EXPECT().HandlersOf("test.event").Return([]events.Handler{handler}).Once()
				factory.EXPECT().OutboxRepository(mock.Anything).Return(storage).Once()
				storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{input.row}, nil).Once()

				return dependencies{storage: storage, factory: factory, registry: registry}
			},
			expect: func(err error) { s.ErrorIs(err, context.Canceled) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := scenario.setup(scenario.args)
			sut := outbox.NewDispatcherJob(&unitOfWorkRows{}, deps.factory, deps.registry, s.cfg, noopLogger{}, rand.New(rand.NewSource(42)))

			err := sut.Run(scenario.args.ctx)

			scenario.expect(err)
		})
	}
}

func (s *DispatcherSuite) TestCalcBackoff() {
	type args struct {
		attempt int
		seed    int64
	}

	scenarios := []struct {
		name   string
		args   []args
		setup  func()
		expect func([]time.Duration)
	}{
		{
			name:  "deve crescer exponencialmente",
			args:  []args{{attempt: 0, seed: 0}, {attempt: 1, seed: 0}, {attempt: 2, seed: 0}},
			setup: func() {},
			expect: func(results []time.Duration) {
				s.Greater(results[1], results[0])
				s.Greater(results[2], results[1])
			},
		},
		{
			name:  "deve aplicar jitter por seed",
			args:  []args{{attempt: 3, seed: 1}, {attempt: 3, seed: 2}},
			setup: func() {},
			expect: func(results []time.Duration) {
				s.NotEqual(results[0], results[1])
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			results := make([]time.Duration, 0, len(scenario.args))
			for _, arg := range scenario.args {
				sut := outbox.NewDispatcherJob(&unitOfWorkRows{}, outboxmocks.NewOutboxRepositoryFactory(s.T()), outboxmocks.NewRegistry(s.T()), s.cfg, noopLogger{}, rand.New(rand.NewSource(arg.seed)))
				results = append(results, sut.CalcBackoff(arg.attempt))
			}

			scenario.expect(results)
		})
	}
}
