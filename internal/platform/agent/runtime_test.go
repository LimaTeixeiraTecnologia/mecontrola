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

func (s *RuntimeTestSuite) TestExecute_InjectsRecentHistoryIntoMessages() {
	threadID := uuid.New()
	history := []memory.Message{
		{Role: memory.RoleUser, Content: "primeira pergunta"},
		{Role: memory.RoleAssistant, Content: "primeira resposta"},
	}
	msgStore := &capturingMessageStore{recent: history}
	ag := &capturingAgent{id: "agent-1", instructions: "Be helpful", result: Result{Content: "ok", Mode: ExecutionModeSync}}

	reg := NewAgentRegistry()
	reg.Register(ag)
	rt := NewAgentRuntime(
		reg,
		&fakeThreadGateway{thread: memory.Thread{ID: threadID, ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
		msgStore,
		&fakeWorkingMemory{},
		&fakeRunStore{},
		s.obs,
	)

	in := InboundRequest{AgentID: "agent-1", ResourceID: "res-1", ThreadID: "thr-1", Message: "nova pergunta", MessageID: "msg-1"}
	_, err := rt.Execute(s.ctx, in)
	s.NoError(err)

	s.Equal(20, msgStore.capturedLimit)
	s.Require().Len(ag.captured.Messages, 4)
	s.Equal("system", ag.captured.Messages[0].Role)
	s.Equal("primeira pergunta", ag.captured.Messages[1].Content)
	s.Equal("primeira resposta", ag.captured.Messages[2].Content)
	s.Equal("nova pergunta", ag.captured.Messages[3].Content)
}

func (s *RuntimeTestSuite) TestExecute_OutcomeOutcomeField_Routed() {
	threadID := uuid.New()

	reg := NewAgentRegistry()
	reg.Register(&fakeAgent{id: "agent-1", instructions: "instr", result: Result{Content: "ok", Mode: ExecutionModeSync}})

	rt := NewAgentRuntime(
		reg,
		&fakeThreadGateway{thread: memory.Thread{ID: threadID, ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
		&fakeMessageStore{},
		&fakeWorkingMemory{},
		&fakeRunStore{},
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
	s.Equal(ToolOutcomeRouted, outcome.Outcome)
	s.Equal("ok", outcome.Content)
}

func (s *RuntimeTestSuite) TestExecute_OutcomeOutcomeField_UsecaseErrorOnEmptyContent() {
	threadID := uuid.New()

	reg := NewAgentRegistry()
	reg.Register(&fakeAgent{id: "agent-1", instructions: "instr", result: Result{Content: "", Mode: ExecutionModeSync}})

	runs := &fakeRunStore{}
	rt := NewAgentRuntime(
		reg,
		&fakeThreadGateway{thread: memory.Thread{ID: threadID, ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
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
	s.Equal(RunStatusFailed, outcome.Status)
	s.Equal(ToolOutcomeUsecaseError, outcome.Outcome)
	s.Empty(outcome.Content)
	s.Require().Len(runs.updated, 1)
	s.Equal(RunStatusFailed, runs.updated[0].Status)
	s.Equal(ToolOutcomeUsecaseError, runs.updated[0].Outcome)
}

func (s *RuntimeTestSuite) TestExecute_RF38_WriteToolGuard_FailsWhenWriteToolErrors() {
	threadID := uuid.New()

	reg := NewAgentRegistry()
	reg.Register(&fakeAgent{
		id:           "agent-1",
		instructions: "instr",
		result: Result{
			Content: "Despesa registrada com sucesso",
			Mode:    ExecutionModeSync,
			ToolCalls: []ToolCallRecord{
				{Tool: "register_expense", Outcome: ToolCallOutcomeError, Content: "ledger error"},
			},
		},
	})

	runs := &fakeRunStore{}
	rt := NewAgentRuntime(
		reg,
		&fakeThreadGateway{thread: memory.Thread{ID: threadID, ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
		&fakeMessageStore{},
		&fakeWorkingMemory{},
		runs,
		s.obs,
		WithWriteToolSet("register_expense"),
	)

	outcome, err := rt.Execute(s.ctx, InboundRequest{
		AgentID:    "agent-1",
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		Message:    "registrar despesa",
		MessageID:  "msg-1",
	})

	s.NoError(err)
	s.Equal(RunStatusFailed, outcome.Status)
	s.Equal(ToolOutcomeUsecaseError, outcome.Outcome)
	s.Empty(outcome.Content)
	s.False(outcome.Succeeded())
}

func (s *RuntimeTestSuite) TestExecute_RF38_WriteToolGuard_SucceedsWhenWriteToolSucceeds() {
	threadID := uuid.New()

	reg := NewAgentRegistry()
	reg.Register(&fakeAgent{
		id:           "agent-1",
		instructions: "instr",
		result: Result{
			Content: "Despesa registrada com sucesso",
			Mode:    ExecutionModeSync,
			ToolCalls: []ToolCallRecord{
				{Tool: "register_expense", Outcome: ToolCallOutcomeSuccess, Content: `{"resourceId":"abc","kind":"transaction","isReplay":false,"outcome":"routed"}`},
			},
		},
	})

	runs := &fakeRunStore{}
	rt := NewAgentRuntime(
		reg,
		&fakeThreadGateway{thread: memory.Thread{ID: threadID, ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
		&fakeMessageStore{},
		&fakeWorkingMemory{},
		runs,
		s.obs,
		WithWriteToolSet("register_expense"),
	)

	outcome, err := rt.Execute(s.ctx, InboundRequest{
		AgentID:    "agent-1",
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		Message:    "registrar despesa",
		MessageID:  "msg-1",
	})

	s.NoError(err)
	s.Equal(RunStatusSucceeded, outcome.Status)
	s.True(outcome.Succeeded())
	s.NotEmpty(outcome.Content)
}

func (s *RuntimeTestSuite) TestExecute_RF39_RoleToolMessagesArePersisted() {
	threadID := uuid.New()

	reg := NewAgentRegistry()
	reg.Register(&fakeAgent{
		id:           "agent-1",
		instructions: "instr",
		result: Result{
			Content: "ok",
			Mode:    ExecutionModeSync,
			ToolCalls: []ToolCallRecord{
				{Tool: "register_expense", Outcome: ToolCallOutcomeSuccess, Content: `{"resourceId":"abc","kind":"transaction","isReplay":false,"outcome":"routed"}`},
			},
		},
	})

	msgs := &recordingMessageStore{}
	rt := NewAgentRuntime(
		reg,
		&fakeThreadGateway{thread: memory.Thread{ID: threadID, ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
		msgs,
		&fakeWorkingMemory{},
		&fakeRunStore{},
		s.obs,
	)

	_, err := rt.Execute(s.ctx, InboundRequest{
		AgentID:    "agent-1",
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		Message:    "registrar despesa",
		MessageID:  "msg-1",
	})

	s.NoError(err)

	var toolMsgs []memory.Message
	for _, m := range msgs.appended {
		if m.Role == memory.RoleTool {
			toolMsgs = append(toolMsgs, m)
		}
	}
	s.Require().Len(toolMsgs, 1)
	s.Equal(memory.RoleTool, toolMsgs[0].Role)
	s.Contains(toolMsgs[0].Content, "transaction")
}

type recordingMessageStore struct {
	appended []memory.Message
}

func (r *recordingMessageStore) Append(_ context.Context, _ uuid.UUID, m memory.Message) error {
	r.appended = append(r.appended, m)
	return nil
}

func (r *recordingMessageStore) Recent(_ context.Context, _ uuid.UUID, _ int) ([]memory.Message, error) {
	return nil, nil
}

type capturingMessageStore struct {
	recent        []memory.Message
	capturedLimit int
}

func (f *capturingMessageStore) Append(_ context.Context, _ uuid.UUID, _ memory.Message) error {
	return nil
}

func (f *capturingMessageStore) Recent(_ context.Context, _ uuid.UUID, limit int) ([]memory.Message, error) {
	f.capturedLimit = limit
	return f.recent, nil
}

type capturingAgent struct {
	id           string
	instructions string
	result       Result
	captured     Request
}

func (f *capturingAgent) ID() string { return f.id }

func (f *capturingAgent) Instructions() string { return f.instructions }

func (f *capturingAgent) Execute(_ context.Context, req Request) (Result, error) {
	f.captured = req
	return f.result, nil
}

func (f *capturingAgent) Stream(_ context.Context, _ Request) (ResultStream, error) {
	return nil, nil
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
