package outbox_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type fakeHandler struct {
	err error
}

func (f *fakeHandler) Handle(_ context.Context, _ events.Event) error { return f.err }

type fakeRegistry struct {
	handlers []events.Handler
}

func (f *fakeRegistry) HandlersOf(_ string) []events.Handler { return f.handlers }

type noopLogger struct{}

func (noopLogger) Debug(_ context.Context, _ string, _ ...observability.Field) {}
func (noopLogger) Info(_ context.Context, _ string, _ ...observability.Field)  {}
func (noopLogger) Warn(_ context.Context, _ string, _ ...observability.Field)  {}
func (noopLogger) Error(_ context.Context, _ string, _ ...observability.Field) {}
func (noopLogger) With(_ ...observability.Field) observability.Logger          { return noopLogger{} }

type fakeUoWRows struct {
	err error
}

func (f *fakeUoWRows) Do(ctx context.Context, fn func(context.Context, database.DBTX) ([]outbox.Row, error), _ ...uow.Option) ([]outbox.Row, error) {
	if f.err != nil {
		return nil, f.err
	}
	return fn(ctx, nil)
}

type DispatcherSuite struct {
	suite.Suite
	storage  *outboxmocks.Storage
	factory  *outboxmocks.OutboxRepositoryFactory
	registry *fakeRegistry
	cfg      configs.OutboxConfig
}

func TestDispatcher(t *testing.T) {
	suite.Run(t, new(DispatcherSuite))
}

func (s *DispatcherSuite) SetupTest() {
	s.storage = outboxmocks.NewStorage(s.T())
	s.factory = outboxmocks.NewOutboxRepositoryFactory(s.T())
	s.registry = &fakeRegistry{}
	s.cfg = configs.OutboxConfig{
		DispatcherBatchSize:      3,
		DispatcherHandlerTimeout: 5 * time.Second,
		RetryMaxAttempts:         15,
		RetryBaseBackoff:         2 * time.Second,
		RetryMaxBackoff:          5 * time.Minute,
	}
}

func (s *DispatcherSuite) makeRow(attempts, maxAttempts int) outbox.Row {
	evt, _ := outbox.NewEvent(outbox.EventInput{
		Type:          "test.event",
		AggregateType: "A",
		AggregateID:   "1",
		Payload:       []byte(`{}`),
	})
	return outbox.Row{Event: evt, Attempts: attempts, MaxAttempts: maxAttempts}
}

func (s *DispatcherSuite) newJob() *outbox.DispatcherJob {
	fakeUoW := &fakeUoWRows{}
	return outbox.NewDispatcherJob(fakeUoW, s.factory, s.registry, s.cfg, noopLogger{}, rand.New(rand.NewSource(42)))
}

func (s *DispatcherSuite) TestRun_Sucesso_1Handler() {
	row := s.makeRow(0, 15)
	s.registry.handlers = []events.Handler{&fakeHandler{}}

	s.factory.EXPECT().OutboxRepository(mock.Anything).Return(s.storage)
	s.storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)
	s.storage.EXPECT().MarkPublished(mock.Anything, row.ID).Return(nil)

	err := s.newJob().Run(context.Background())
	s.NoError(err)
}

func (s *DispatcherSuite) TestRun_Sucesso_3Handlers() {
	row := s.makeRow(0, 15)
	s.registry.handlers = []events.Handler{&fakeHandler{}, &fakeHandler{}, &fakeHandler{}}

	s.factory.EXPECT().OutboxRepository(mock.Anything).Return(s.storage)
	s.storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)
	s.storage.EXPECT().MarkPublished(mock.Anything, row.ID).Return(nil)

	err := s.newJob().Run(context.Background())
	s.NoError(err)
}

func (s *DispatcherSuite) TestRun_FalhaParcial_RetryComBackoff() {
	row := s.makeRow(0, 15)
	handlerErr := errors.New("handler failed")
	s.registry.handlers = []events.Handler{&fakeHandler{}, &fakeHandler{err: handlerErr}, &fakeHandler{}}

	s.factory.EXPECT().OutboxRepository(mock.Anything).Return(s.storage)
	s.storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)
	s.storage.EXPECT().MarkPendingRetry(
		mock.Anything, row.ID, handlerErr.Error(), mock.AnythingOfType("time.Time"),
	).Return(nil)

	err := s.newJob().Run(context.Background())
	s.NoError(err)
}

func (s *DispatcherSuite) TestRun_MaxAttempts_MarkFailed() {
	row := s.makeRow(14, 15)
	handlerErr := errors.New("fatal handler")
	s.registry.handlers = []events.Handler{&fakeHandler{err: handlerErr}}

	s.factory.EXPECT().OutboxRepository(mock.Anything).Return(s.storage)
	s.storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)
	s.storage.EXPECT().MarkFailed(mock.Anything, row.ID, handlerErr.Error()).Return(nil)

	err := s.newJob().Run(context.Background())
	s.NoError(err)
}

func (s *DispatcherSuite) TestRun_ZeroHandlers_MarkFailed() {
	row := s.makeRow(0, 15)
	s.registry.handlers = nil

	s.factory.EXPECT().OutboxRepository(mock.Anything).Return(s.storage)
	s.storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)
	s.storage.EXPECT().MarkFailed(mock.Anything, row.ID, "no handlers registered").Return(nil)

	err := s.newJob().Run(context.Background())
	s.NoError(err)
}

func (s *DispatcherSuite) TestRun_CtxCancelado_AbortaSemPanic() {
	row := s.makeRow(0, 15)
	s.registry.handlers = []events.Handler{&fakeHandler{}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.factory.EXPECT().OutboxRepository(mock.Anything).Return(s.storage)
	s.storage.EXPECT().ClaimBatch(mock.Anything, mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)

	err := s.newJob().Run(ctx)
	s.ErrorIs(err, context.Canceled)
}

func (s *DispatcherSuite) TestBackoff_CresceExponencialmente() {
	rng := rand.New(rand.NewSource(0))
	fakeUoW := &fakeUoWRows{}
	d := outbox.NewDispatcherJob(fakeUoW, s.factory, s.registry, s.cfg, noopLogger{}, rng)

	b0 := d.CalcBackoff(0)
	b1 := d.CalcBackoff(1)
	b2 := d.CalcBackoff(2)

	s.Greater(int64(b1), int64(b0), "backoff 1 deve ser maior que 0")
	s.Greater(int64(b2), int64(b1), "backoff 2 deve ser maior que 1")
}

func (s *DispatcherSuite) TestBackoff_JitterAplicado() {
	rng1 := rand.New(rand.NewSource(1))
	rng2 := rand.New(rand.NewSource(2))

	fakeUoW := &fakeUoWRows{}
	d1 := outbox.NewDispatcherJob(fakeUoW, s.factory, s.registry, s.cfg, noopLogger{}, rng1)
	d2 := outbox.NewDispatcherJob(fakeUoW, s.factory, s.registry, s.cfg, noopLogger{}, rng2)

	b1 := d1.CalcBackoff(3)
	b2 := d2.CalcBackoff(3)

	s.NotEqual(b1, b2, "jitters diferentes devem produzir durações diferentes")
}
