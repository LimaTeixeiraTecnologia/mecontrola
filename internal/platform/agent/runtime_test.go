package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type RuntimeTestSuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestRuntimeTestSuite(t *testing.T) {
	suite.Run(t, new(RuntimeTestSuite))
}

func (s *RuntimeTestSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
}

func (s *RuntimeTestSuite) TestExecute_Success() {
	type args struct {
		in InboundRequest
	}
	type dependencies struct {
		threads  *fakeThreadGateway
		messages *fakeMessageStore
		wm       *fakeWorkingMemory
		runs     *fakeRunStore
		agent    *fakeAgent
	}

	threadID := uuid.New()

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(outcome Outcome, err error)
	}{
		{
			name: "deve executar Thread->Run com sucesso",
			args: args{in: InboundRequest{
				AgentID:    "agent-1",
				ResourceID: "res-1",
				ThreadID:   "thr-1",
				Message:    "hello",
				MessageID:  "msg-1",
			}},
			dependencies: dependencies{
				threads:  &fakeThreadGateway{thread: memory.Thread{ID: threadID, ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
				messages: &fakeMessageStore{},
				wm:       &fakeWorkingMemory{},
				runs:     &fakeRunStore{},
				agent:    &fakeAgent{id: "agent-1", instructions: "Be helpful", result: Result{Content: "world", Mode: ExecutionModeSync}},
			},
			expect: func(outcome Outcome, err error) {
				s.NoError(err)
				s.Equal(RunStatusSucceeded, outcome.Status)
				s.Equal("world", outcome.Content)
				s.NotEqual(uuid.Nil, outcome.RunID)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			reg := NewAgentRegistry()
			reg.Register(scenario.dependencies.agent)
			rt := NewAgentRuntime(
				reg,
				scenario.dependencies.threads,
				scenario.dependencies.messages,
				scenario.dependencies.wm,
				scenario.dependencies.runs,
				s.obs,
			)
			outcome, err := rt.Execute(s.ctx, scenario.args.in)
			scenario.expect(outcome, err)
		})
	}
}

func (s *RuntimeTestSuite) TestExecute_ValidationError() {
	type args struct {
		in InboundRequest
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(outcome Outcome, err error)
	}{
		{
			name:   "deve falhar com agent_id vazio",
			args:   args{in: InboundRequest{ResourceID: "res", ThreadID: "thr", Message: "hi"}},
			expect: func(outcome Outcome, err error) { s.Error(err) },
		},
		{
			name:   "deve falhar com resource_id vazio",
			args:   args{in: InboundRequest{AgentID: "ag", ThreadID: "thr", Message: "hi"}},
			expect: func(outcome Outcome, err error) { s.Error(err) },
		},
		{
			name:   "deve falhar com thread_id vazio",
			args:   args{in: InboundRequest{AgentID: "ag", ResourceID: "res", Message: "hi"}},
			expect: func(outcome Outcome, err error) { s.Error(err) },
		},
		{
			name:   "deve falhar com message vazio",
			args:   args{in: InboundRequest{AgentID: "ag", ResourceID: "res", ThreadID: "thr"}},
			expect: func(outcome Outcome, err error) { s.Error(err) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			rt := NewAgentRuntime(
				NewAgentRegistry(),
				&fakeThreadGateway{},
				&fakeMessageStore{},
				&fakeWorkingMemory{},
				&fakeRunStore{},
				s.obs,
			)
			outcome, err := rt.Execute(s.ctx, scenario.args.in)
			scenario.expect(outcome, err)
		})
	}
}

func (s *RuntimeTestSuite) TestExecute_ThreadError() {
	type args struct {
		in InboundRequest
	}
	type dependencies struct {
		threads *fakeThreadGateway
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(outcome Outcome, err error)
	}{
		{
			name: "deve retornar erro quando thread falha",
			args: args{in: InboundRequest{
				AgentID:    "agent-1",
				ResourceID: "res-1",
				ThreadID:   "thr-1",
				Message:    "hello",
			}},
			dependencies: dependencies{
				threads: &fakeThreadGateway{err: errors.New("db error")},
			},
			expect: func(outcome Outcome, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			rt := NewAgentRuntime(
				NewAgentRegistry(),
				scenario.dependencies.threads,
				&fakeMessageStore{},
				&fakeWorkingMemory{},
				&fakeRunStore{},
				s.obs,
			)
			outcome, err := rt.Execute(s.ctx, scenario.args.in)
			scenario.expect(outcome, err)
		})
	}
}

func (s *RuntimeTestSuite) TestExecute_AgentNotFound() {
	type args struct {
		in InboundRequest
	}
	type dependencies struct {
		threads *fakeThreadGateway
		runs    *fakeRunStore
	}

	threadID := uuid.New()

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(outcome Outcome, err error)
	}{
		{
			name: "deve retornar erro quando agente nao encontrado",
			args: args{in: InboundRequest{
				AgentID:    "non-existent",
				ResourceID: "res-1",
				ThreadID:   "thr-1",
				Message:    "hello",
			}},
			dependencies: dependencies{
				threads: &fakeThreadGateway{thread: memory.Thread{ID: threadID, ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
				runs:    &fakeRunStore{},
			},
			expect: func(outcome Outcome, err error) {
				s.Error(err)
				s.True(errors.Is(err, ErrAgentNotFound))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			rt := NewAgentRuntime(
				NewAgentRegistry(),
				scenario.dependencies.threads,
				&fakeMessageStore{},
				&fakeWorkingMemory{},
				scenario.dependencies.runs,
				s.obs,
			)
			outcome, err := rt.Execute(s.ctx, scenario.args.in)
			scenario.expect(outcome, err)
		})
	}
}

func (s *RuntimeTestSuite) TestExecute_AgentExecuteError() {
	type args struct {
		in InboundRequest
	}
	type dependencies struct {
		agent *fakeAgent
	}

	threadID := uuid.New()
	thread := memory.Thread{ID: threadID, ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(outcome Outcome, err error)
	}{
		{
			name: "deve retornar erro quando agent.Execute falha",
			args: args{in: InboundRequest{
				AgentID:    "agent-1",
				ResourceID: "res-1",
				ThreadID:   "thr-1",
				Message:    "hello",
			}},
			dependencies: dependencies{
				agent: &fakeAgent{id: "agent-1", instructions: "Be helpful", err: errors.New("agent failed")},
			},
			expect: func(outcome Outcome, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			reg := NewAgentRegistry()
			reg.Register(scenario.dependencies.agent)
			rt := NewAgentRuntime(
				reg,
				&fakeThreadGateway{thread: thread},
				&fakeMessageStore{},
				&fakeWorkingMemory{},
				&fakeRunStore{},
				s.obs,
			)
			outcome, err := rt.Execute(s.ctx, scenario.args.in)
			scenario.expect(outcome, err)
		})
	}
}

type fakeThreadGateway struct {
	thread memory.Thread
	err    error
}

func (f *fakeThreadGateway) GetOrCreate(_ context.Context, _, _ string) (memory.Thread, error) {
	return f.thread, f.err
}

type fakeMessageStore struct{}

func (f *fakeMessageStore) Append(_ context.Context, _ uuid.UUID, _ memory.Message) error {
	return nil
}

func (f *fakeMessageStore) Recent(_ context.Context, _ uuid.UUID, _ int) ([]memory.Message, error) {
	return nil, nil
}

type fakeWorkingMemory struct {
	content string
	err     error
}

func (f *fakeWorkingMemory) Get(_ context.Context, _ string) (string, error) {
	return f.content, f.err
}

func (f *fakeWorkingMemory) Upsert(_ context.Context, _, _ string) error {
	return nil
}

type fakeRunStore struct {
	insertErr error
	updateErr error
	updated   []Run
}

func (f *fakeRunStore) Insert(_ context.Context, _ Run) error {
	return f.insertErr
}

func (f *fakeRunStore) Update(_ context.Context, run Run) error {
	f.updated = append(f.updated, run)
	return f.updateErr
}

func (f *fakeRunStore) Load(_ context.Context, _ uuid.UUID) (Run, error) {
	return Run{}, ErrRunNotFound
}
