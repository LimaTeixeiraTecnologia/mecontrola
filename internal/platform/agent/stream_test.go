package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	llmmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm/mocks"
)

type StreamTestSuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestStreamTestSuite(t *testing.T) {
	suite.Run(t, new(StreamTestSuite))
}

func (s *StreamTestSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
}

func (s *StreamTestSuite) TestDrain_ContextCancelMidStreamExitsGoroutine() {
	deltas := make(chan string, 200)
	for i := 0; i < 200; i++ {
		deltas <- "x"
	}

	ts := &blockingTokenStream{deltas: deltas}
	ctx, cancel := context.WithCancel(context.Background())

	rs := newResultStream(ctx, ts, nil, NoopHooks{}, "agent-1")

	cancel()

	select {
	case <-rs.done:
	case <-time.After(time.Second):
		s.Fail("drain goroutine did not exit after context cancellation")
	}

	_, err := rs.Result(context.Background())
	s.Error(err)
}

func (s *StreamTestSuite) TestStream_InvokesBeforeAndAfterExecuteExactlyOnce() {
	hooks := &recordingHooks{after: make(chan struct{})}
	provider := llmmocks.NewProvider(s.T())
	a := NewAgent("agent-1", "instr", provider, s.obs, WithHooks(hooks))

	ts := llmmocks.NewTokenStream(s.T())
	ch := make(chan string, 2)
	ch <- "hel"
	ch <- "lo"
	close(ch)
	ts.EXPECT().Deltas().Return((<-chan string)(ch)).Once()
	ts.EXPECT().Err().Return(nil).Once()
	provider.EXPECT().
		Stream(mock.Anything, mock.AnythingOfType("llm.Request")).
		Return(ts, nil).
		Once()

	rs, err := a.Stream(s.ctx, Request{
		AgentID:  "agent-1",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	s.NoError(err)

	result, resErr := rs.Result(s.ctx)
	s.NoError(resErr)
	s.Equal("hello", result.Content)

	select {
	case <-hooks.after:
	case <-time.After(time.Second):
		s.Fail("AfterExecute was not invoked")
	}

	s.Equal(1, hooks.beforeCount())
	s.Equal(1, hooks.afterCount())
}

type blockingTokenStream struct {
	deltas chan string
}

func (b *blockingTokenStream) Deltas() <-chan string { return b.deltas }

func (b *blockingTokenStream) Close() error { return nil }

func (b *blockingTokenStream) Err() error { return nil }

type recordingHooks struct {
	mu     sync.Mutex
	before int
	afterN int
	after  chan struct{}
}

func (h *recordingHooks) BeforeExecute(ctx context.Context, _ string, _ Request) context.Context {
	h.mu.Lock()
	h.before++
	h.mu.Unlock()
	return ctx
}

func (h *recordingHooks) AfterExecute(_ context.Context, _ string, _ Result, _ error) {
	h.mu.Lock()
	h.afterN++
	h.mu.Unlock()
	close(h.after)
}

func (h *recordingHooks) BeforeTool(ctx context.Context, _, _ string) context.Context {
	return ctx
}

func (h *recordingHooks) AfterTool(_ context.Context, _, _ string, _ []byte, _ error) {}

func (h *recordingHooks) beforeCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.before
}

func (h *recordingHooks) afterCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.afterN
}
