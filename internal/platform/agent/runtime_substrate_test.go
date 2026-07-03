package agent

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	llmmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type SubstrateSuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestSubstrateSuite(t *testing.T) {
	suite.Run(t, new(SubstrateSuite))
}

func (s *SubstrateSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
}

func (s *SubstrateSuite) TestRF37_IdentityInjectedServerSide() {
	type capturedIdentity struct {
		ResourceID string
		MessageID  string
		ItemSeq    int
	}
	var got capturedIdentity

	captureTool := tool.NewTool[map[string]any, map[string]any](
		"test_write",
		"capture identity from context",
		llm.Schema{Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []string{},
		}},
		llm.Schema{Schema: map[string]any{"type": "object"}},
		func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			resourceID, messageID, itemSeq, ok := InboundIdentityFromContext(ctx)
			if ok {
				got = capturedIdentity{ResourceID: resourceID, MessageID: messageID, ItemSeq: itemSeq}
			}
			return map[string]any{}, nil
		},
	)

	provider := llmmocks.NewProvider(s.T())
	provider.EXPECT().
		Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
		Return(llm.Response{
			ToolCalls: []llm.ToolCall{{ID: "tc1", FunctionName: "test_write", ArgumentsJSON: map[string]any{}}},
		}, nil).Once()
	provider.EXPECT().
		Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
		Return(llm.Response{Content: "ok"}, nil).Once()

	a := NewAgent("a1", "instr", provider, s.obs, WithTools(captureTool))

	identity := &toolIdentity{userID: "user-123", wamid: "wamid-456"}
	ctx := context.WithValue(s.ctx, identityKey{}, identity)

	result, err := a.Execute(ctx, Request{
		Messages: []llm.Message{{Role: "user", Content: "register"}},
	})

	s.NoError(err)
	s.Equal("ok", result.Content)
	s.Equal("user-123", got.ResourceID)
	s.Equal("wamid-456", got.MessageID)
	s.Equal(0, got.ItemSeq)
	s.Require().Len(result.ToolCalls, 1)
	s.Equal("test_write", result.ToolCalls[0].Tool)
	s.Equal(ToolCallOutcomeSuccess, result.ToolCalls[0].Outcome)
}

func (s *SubstrateSuite) TestRF38_AntiSimGuard_WriteToolFailed_StatusFailed() {
	threadID := uuid.New()

	ag := &fakeAgent{
		id:           "agent-1",
		instructions: "instr",
		result: Result{
			Content: "Despesa registrada com sucesso!",
			ToolCalls: []ToolCallRecord{
				{Tool: "register_expense", Outcome: ToolCallOutcomeError, Content: "usecase error"},
			},
		},
	}

	reg := NewAgentRegistry()
	reg.Register(ag)
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
	s.Require().Len(runs.updated, 1)
	s.Equal(RunStatusFailed, runs.updated[0].Status)
}

func (s *SubstrateSuite) TestRF38_AntiSimGuard_WriteToolSucceeded_StatusSucceeded() {
	threadID := uuid.New()

	ag := &fakeAgent{
		id:           "agent-1",
		instructions: "instr",
		result: Result{
			Content: "Despesa registrada!",
			ToolCalls: []ToolCallRecord{
				{Tool: "register_expense", Outcome: ToolCallOutcomeSuccess, Content: `{"resourceId":"abc"}`},
			},
		},
	}

	reg := NewAgentRegistry()
	reg.Register(ag)

	rt := NewAgentRuntime(
		reg,
		&fakeThreadGateway{thread: memory.Thread{ID: threadID, ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
		&fakeMessageStore{},
		&fakeWorkingMemory{},
		&fakeRunStore{},
		s.obs,
		WithWriteToolSet("register_expense"),
	)

	outcome, err := rt.Execute(s.ctx, InboundRequest{
		AgentID:    "agent-1",
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		Message:    "registrar",
		MessageID:  "msg-1",
	})

	s.NoError(err)
	s.Equal(RunStatusSucceeded, outcome.Status)
	s.Equal(ToolOutcomeRouted, outcome.Outcome)
}

func (s *SubstrateSuite) TestRF39_RoleToolPersisted() {
	threadID := uuid.New()

	ag := &fakeAgent{
		id:           "agent-1",
		instructions: "instr",
		result: Result{
			Content: "ok",
			ToolCalls: []ToolCallRecord{
				{Tool: "register_expense", Outcome: ToolCallOutcomeSuccess, Content: `{"resourceId":"tx-1"}`},
			},
		},
	}

	reg := NewAgentRegistry()
	reg.Register(ag)

	msgStore := &appendingMessageStore{}

	rt := NewAgentRuntime(
		reg,
		&fakeThreadGateway{thread: memory.Thread{ID: threadID, ResourceID: "res-1", ThreadID: "thr-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}},
		msgStore,
		&fakeWorkingMemory{},
		&fakeRunStore{},
		s.obs,
	)

	_, err := rt.Execute(s.ctx, InboundRequest{
		AgentID:    "agent-1",
		ResourceID: "res-1",
		ThreadID:   "thr-1",
		Message:    "registrar",
		MessageID:  "msg-1",
	})

	s.NoError(err)

	roles := make([]memory.MessageRole, len(msgStore.appended))
	for i, m := range msgStore.appended {
		roles[i] = m.Role
	}
	s.Contains(roles, memory.RoleTool)

	var toolMsg memory.Message
	for _, m := range msgStore.appended {
		if m.Role == memory.RoleTool {
			toolMsg = m
			break
		}
	}
	s.Equal(`{"resourceId":"tx-1"}`, toolMsg.Content)
}

type appendingMessageStore struct {
	appended []memory.Message
	recent   []memory.Message
}

func (f *appendingMessageStore) Append(_ context.Context, _ uuid.UUID, msg memory.Message) error {
	f.appended = append(f.appended, msg)
	return nil
}

func (f *appendingMessageStore) Recent(_ context.Context, _ uuid.UUID, _ int) ([]memory.Message, error) {
	return f.recent, nil
}
