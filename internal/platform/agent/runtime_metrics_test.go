package agent

import (
	"context"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	llmmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

func (s *RuntimeTestSuite) TestExecute_EmitsRunMetricExactlyOnce() {
	provider := llmmocks.NewProvider(s.T())
	provider.EXPECT().
		Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
		Return(llm.Response{Content: "world"}, nil).
		Once()

	registry := NewAgentRegistry()
	registry.Register(NewAgent("agent-1", "Be helpful", provider, s.obs))

	runs := &fakeRunStore{}
	rt := NewAgentRuntime(
		registry,
		&fakeThreadGateway{thread: memory.Thread{ID: uuid.New(), ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
		&fakeMessageStore{},
		&fakeWorkingMemory{},
		runs,
		s.obs,
	)

	outcome, err := rt.Execute(s.ctx, InboundRequest{
		AgentID:    "agent-1",
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		Message:    "hello",
		MessageID:  "msg-1",
	})

	s.NoError(err)
	s.Equal(RunStatusSucceeded, outcome.Status)

	metrics := s.obs.Metrics().(*fake.FakeMetrics)
	s.Len(metrics.GetCounter("agent_runs_total").GetValues(), 1)
	s.Len(metrics.GetHistogram("agent_run_duration_seconds").GetValues(), 1)

	s.Require().NotEmpty(runs.updated)
	last := runs.updated[len(runs.updated)-1]
	s.Equal(ToolOutcomeRouted, last.Outcome)
	s.Equal(RunStatusSucceeded, last.Status)
}

func (s *RuntimeTestSuite) TestExecute_ResolveFailure_EmitsRunMetricExactlyOnceFailed() {
	runs := &fakeRunStore{}
	rt := NewAgentRuntime(
		NewAgentRegistry(),
		&fakeThreadGateway{thread: memory.Thread{ID: uuid.New(), ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
		&fakeMessageStore{},
		&fakeWorkingMemory{},
		runs,
		s.obs,
	)

	_, err := rt.Execute(s.ctx, InboundRequest{
		AgentID:    "non-existent",
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		Message:    "hello",
		MessageID:  "msg-1",
	})
	s.Error(err)

	metrics := s.obs.Metrics().(*fake.FakeMetrics)
	values := metrics.GetCounter("agent_runs_total").GetValues()
	s.Require().Len(values, 1)

	var status string
	for _, f := range values[0].Fields {
		if f.Key == "status" {
			status = f.StringValue()
		}
	}
	s.Equal(RunStatusFailed.String(), status)

	s.Require().NotEmpty(runs.updated)
	s.Equal(RunStatusFailed, runs.updated[len(runs.updated)-1].Status)
}

func (s *RuntimeTestSuite) TestExecute_InjectsWorkingMemoryIntoSystemPrompt() {
	var captured llm.Request
	provider := llmmocks.NewProvider(s.T())
	provider.EXPECT().
		Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
		Run(func(_ context.Context, req llm.Request) { captured = req }).
		Return(llm.Response{Content: "world"}, nil).
		Once()

	registry := NewAgentRegistry()
	registry.Register(NewAgent("agent-1", "Be helpful", provider, s.obs))

	rt := NewAgentRuntime(
		registry,
		&fakeThreadGateway{thread: memory.Thread{ID: uuid.New(), ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
		&fakeMessageStore{},
		&fakeWorkingMemory{content: "User prefers BRL and weekly summaries."},
		&fakeRunStore{},
		s.obs,
	)

	_, err := rt.Execute(s.ctx, InboundRequest{
		AgentID:    "agent-1",
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		Message:    "hello",
		MessageID:  "msg-1",
	})
	s.NoError(err)

	var system string
	for _, m := range captured.Messages {
		if m.Role == "system" {
			system = m.Content
		}
	}
	s.Contains(system, "## Working Memory")
	s.Contains(system, "User prefers BRL and weekly summaries.")
	s.True(strings.Contains(system, "Be helpful"))
}
