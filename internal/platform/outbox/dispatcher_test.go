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

type DispatcherSuite struct {
	suite.Suite
	storage  *outboxmocks.Storage
	registry *fakeRegistry
	cfg      configs.OutboxConfig
}

func TestDispatcher(t *testing.T) {
	suite.Run(t, new(DispatcherSuite))
}

func (s *DispatcherSuite) SetupTest() {
	s.storage = outboxmocks.NewStorage(s.T())
	s.registry = &fakeRegistry{}
	s.cfg = configs.OutboxConfig{
		DispatcherBatchSize:      3,
		DispatcherHandlerTimeout: 5 * time.Second,
		RetryMaxAttempts:         15,
		RetryBaseBackoff:         2 * time.Second,
		RetryMaxBackoff:          5 * time.Minute,
	}
}

func (s *DispatcherSuite) newDispatcher() *outbox.OutboxDispatcher {
	return outbox.NewOutboxDispatcher(s.storage, s.registry, s.cfg, noopLogger{}, rand.New(rand.NewSource(42)))
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

func (s *DispatcherSuite) TestRunOnce_Sucesso_1Handler() {
	row := s.makeRow(0, 15)
	s.registry.handlers = []events.Handler{&fakeHandler{}}

	s.storage.EXPECT().ClaimBatch(context.Background(), mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)
	s.storage.EXPECT().MarkPublished(context.Background(), row.ID).Return(nil)

	err := s.newDispatcher().RunOnce(context.Background())
	s.NoError(err)
}

func (s *DispatcherSuite) TestRunOnce_Sucesso_3Handlers() {
	row := s.makeRow(0, 15)
	s.registry.handlers = []events.Handler{&fakeHandler{}, &fakeHandler{}, &fakeHandler{}}

	s.storage.EXPECT().ClaimBatch(context.Background(), mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)
	s.storage.EXPECT().MarkPublished(context.Background(), row.ID).Return(nil)

	err := s.newDispatcher().RunOnce(context.Background())
	s.NoError(err)
}

func (s *DispatcherSuite) TestRunOnce_FalhaParcial_RetryComBackoff() {
	row := s.makeRow(0, 15)
	handlerErr := errors.New("handler failed")
	s.registry.handlers = []events.Handler{&fakeHandler{}, &fakeHandler{err: handlerErr}, &fakeHandler{}}

	s.storage.EXPECT().ClaimBatch(context.Background(), mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)
	s.storage.EXPECT().MarkPendingRetry(
		context.Background(), row.ID, handlerErr.Error(), mock.AnythingOfType("time.Time"),
	).Return(nil)

	err := s.newDispatcher().RunOnce(context.Background())
	s.NoError(err)
}

func (s *DispatcherSuite) TestRunOnce_MaxAttempts_MarkFailed() {
	row := s.makeRow(14, 15)
	handlerErr := errors.New("fatal handler")
	s.registry.handlers = []events.Handler{&fakeHandler{err: handlerErr}}

	s.storage.EXPECT().ClaimBatch(context.Background(), mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)
	s.storage.EXPECT().MarkFailed(context.Background(), row.ID, handlerErr.Error()).Return(nil)

	err := s.newDispatcher().RunOnce(context.Background())
	s.NoError(err)
}

func (s *DispatcherSuite) TestRunOnce_ZeroHandlers_MarkFailed() {
	row := s.makeRow(0, 15)
	s.registry.handlers = nil

	s.storage.EXPECT().ClaimBatch(context.Background(), mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)
	s.storage.EXPECT().MarkFailed(context.Background(), row.ID, "no handlers registered").Return(nil)

	err := s.newDispatcher().RunOnce(context.Background())
	s.NoError(err)
}

func (s *DispatcherSuite) TestRunOnce_CtxCancelado_AbortaSemPanic() {
	row := s.makeRow(0, 15)
	s.registry.handlers = []events.Handler{&fakeHandler{}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.storage.EXPECT().ClaimBatch(ctx, mock.AnythingOfType("string"), 3).Return([]outbox.Row{row}, nil)

	err := s.newDispatcher().RunOnce(ctx)
	s.ErrorIs(err, context.Canceled)
}

func (s *DispatcherSuite) TestBackoff_CresceExponencialmente() {
	rng := rand.New(rand.NewSource(0))
	d := outbox.NewOutboxDispatcher(s.storage, s.registry, s.cfg, noopLogger{}, rng)

	b0 := d.CalcBackoff(0)
	b1 := d.CalcBackoff(1)
	b2 := d.CalcBackoff(2)

	s.Greater(int64(b1), int64(b0), "backoff 1 deve ser maior que 0")
	s.Greater(int64(b2), int64(b1), "backoff 2 deve ser maior que 1")
}

func (s *DispatcherSuite) TestBackoff_JitterAplicado() {
	rng1 := rand.New(rand.NewSource(1))
	rng2 := rand.New(rand.NewSource(2))

	d1 := outbox.NewOutboxDispatcher(s.storage, s.registry, s.cfg, noopLogger{}, rng1)
	d2 := outbox.NewOutboxDispatcher(s.storage, s.registry, s.cfg, noopLogger{}, rng2)

	b1 := d1.CalcBackoff(3)
	b2 := d2.CalcBackoff(3)

	s.NotEqual(b1, b2, "jitters diferentes devem produzir durações diferentes")
}
