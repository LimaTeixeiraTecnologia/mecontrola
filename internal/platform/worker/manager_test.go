package worker_test

import (
	"context"
	"io"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
)

type ManagerSuite struct {
	suite.Suite
}

func TestManagerSuite(t *testing.T) {
	suite.Run(t, new(ManagerSuite))
}

func (s *ManagerSuite) TestStartStop_Sucesso() {
	cfg := worker.Config{ShutdownTimeout: 5 * time.Second}
	j := &fakeJob{name: "job1"}
	c := newFakeConsumer("consumer1")

	m := worker.NewManager(cfg, []worker.Job{j}, []worker.Consumer{c}, noopLogger())

	err := m.Start(context.Background())
	s.NoError(err)

	time.Sleep(50 * time.Millisecond)

	err = m.Stop(context.Background())
	s.NoError(err)
}

func (s *ManagerSuite) TestStart_NomeDuplicadoEmJobs() {
	cfg := worker.Config{ShutdownTimeout: 5 * time.Second}
	j1 := &fakeJob{name: "mesmo"}
	j2 := &fakeJob{name: "mesmo"}

	m := worker.NewManager(cfg, []worker.Job{j1, j2}, nil, noopLogger())
	err := m.Start(context.Background())
	s.Error(err)
}

func (s *ManagerSuite) TestStart_NomeDuplicadoEmConsumers() {
	cfg := worker.Config{ShutdownTimeout: 5 * time.Second}
	c1 := newFakeConsumer("mesmo")
	c2 := newFakeConsumer("mesmo")

	m := worker.NewManager(cfg, nil, []worker.Consumer{c1, c2}, noopLogger())
	err := m.Start(context.Background())
	s.Error(err)
}

func (s *ManagerSuite) TestStop_ChamamConsumersStop() {
	cfg := worker.Config{ShutdownTimeout: 5 * time.Second}
	c := newFakeConsumer("c1")

	m := worker.NewManager(cfg, nil, []worker.Consumer{c}, noopLogger())
	err := m.Start(context.Background())
	s.NoError(err)

	time.Sleep(30 * time.Millisecond)

	err = m.Stop(context.Background())
	s.NoError(err)

	select {
	case <-c.stopped:
	case <-time.After(2 * time.Second):
		s.Fail("consumer.Stop não foi chamado")
	}
}

func (s *ManagerSuite) TestSemGoroutineLeak() {
	before := runtime.NumGoroutine()

	cfg := worker.Config{ShutdownTimeout: 5 * time.Second}
	c := newFakeConsumer("c1")
	m := worker.NewManager(cfg, nil, []worker.Consumer{c}, noopLogger())

	err := m.Start(context.Background())
	s.NoError(err)

	err = m.Stop(context.Background())
	s.NoError(err)

	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()
	s.LessOrEqual(after, before+2)
}

type fakeJob struct{ name string }

func (j *fakeJob) Name() string                { return j.name }
func (j *fakeJob) Schedule() string            { return "@every 1h" }
func (j *fakeJob) Run(_ context.Context) error { return nil }

type fakeConsumer struct {
	name    string
	started chan struct{}
	stopped chan struct{}
}

func newFakeConsumer(name string) *fakeConsumer {
	return &fakeConsumer{
		name:    name,
		started: make(chan struct{}),
		stopped: make(chan struct{}, 1),
	}
}

func (c *fakeConsumer) Name() string       { return c.name }
func (c *fakeConsumer) Technology() string { return "fake" }

func (c *fakeConsumer) Start(ctx context.Context) error {
	select {
	case c.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return nil
}

func (c *fakeConsumer) Stop(_ context.Context) error {
	select {
	case c.stopped <- struct{}{}:
	default:
	}
	return nil
}

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
