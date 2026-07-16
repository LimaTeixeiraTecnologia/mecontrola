package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type resumerTestState struct {
	Value string
}

type mockResumerEngine struct {
	mock.Mock
}

func (m *mockResumerEngine) Start(ctx context.Context, def workflow.Definition[resumerTestState], key string, initial resumerTestState) (workflow.RunResult[resumerTestState], error) {
	args := m.Called(ctx, def, key, initial)
	return args.Get(0).(workflow.RunResult[resumerTestState]), args.Error(1)
}

func (m *mockResumerEngine) Resume(ctx context.Context, def workflow.Definition[resumerTestState], key string, resume []byte) (workflow.RunResult[resumerTestState], error) {
	args := m.Called(ctx, def, key, resume)
	return args.Get(0).(workflow.RunResult[resumerTestState]), args.Error(1)
}

func (m *mockResumerEngine) LoadLatestState(ctx context.Context, def workflow.Definition[resumerTestState], key string) (resumerTestState, workflow.Snapshot, bool, error) {
	args := m.Called(ctx, def, key)
	return args.Get(0).(resumerTestState), args.Get(1).(workflow.Snapshot), args.Bool(2), args.Error(3)
}

type WorkflowResumerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestWorkflowResumerSuite(t *testing.T) {
	suite.Run(t, new(WorkflowResumerSuite))
}

func (s *WorkflowResumerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *WorkflowResumerSuite) TestNewWorkflowResumer_ErroQuandoWorkflowNaoRegistrado() {
	registry := agent.NewWorkflowRegistry[resumerTestState]()
	engine := &mockResumerEngine{}

	resumer, err := NewWorkflowResumer(
		"unknown-workflow",
		registry,
		engine,
		func(resourceID, threadID string) string { return resourceID + ":" + threadID },
		func(_ context.Context, _ workflow.Engine[resumerTestState], _ workflow.Definition[resumerTestState], _, _ string) (bool, string, error) {
			return false, "", nil
		},
	)

	s.Error(err)
	s.Nil(resumer)
}

func (s *WorkflowResumerSuite) TestResume_DelegaParaContinueFnComChaveCorreta() {
	registry := agent.NewWorkflowRegistry[resumerTestState]()
	def := workflow.Definition[resumerTestState]{ID: "test-workflow"}
	registry.Register(def)

	engine := &mockResumerEngine{}
	engine.On("Resume", mock.Anything, def, "user-1:+5511:test-workflow", mock.Anything).
		Return(workflow.RunResult[resumerTestState]{
			Status: workflow.RunStatusSucceeded,
			State:  resumerTestState{Value: "done"},
		}, nil).Once()

	resumer, err := NewWorkflowResumer(
		"test-workflow",
		registry,
		engine,
		func(resourceID, threadID string) string { return resourceID + ":" + threadID + ":test-workflow" },
		func(ctx context.Context, eng workflow.Engine[resumerTestState], d workflow.Definition[resumerTestState], key, _ string) (bool, string, error) {
			result, resumeErr := eng.Resume(ctx, d, key, nil)
			if resumeErr != nil {
				return false, "", resumeErr
			}
			return true, result.State.Value, nil
		},
	)
	s.Require().NoError(err)
	s.Equal("test-workflow", resumer.WorkflowID())

	handled, reply, resumeErr := resumer.Resume(s.ctx, "user-1", "+5511", "sim")
	s.NoError(resumeErr)
	s.True(handled)
	s.Equal("done", reply)
	engine.AssertExpectations(s.T())
}

func (s *WorkflowResumerSuite) TestResume_PropagaErroDoContinueFn() {
	registry := agent.NewWorkflowRegistry[resumerTestState]()
	def := workflow.Definition[resumerTestState]{ID: "test-workflow"}
	registry.Register(def)

	engine := &mockResumerEngine{}
	engine.On("Resume", mock.Anything, def, "user-2:+5511:test-workflow", mock.Anything).
		Return(workflow.RunResult[resumerTestState]{}, errors.New("engine falhou")).Once()

	resumer, err := NewWorkflowResumer(
		"test-workflow",
		registry,
		engine,
		func(resourceID, threadID string) string { return resourceID + ":" + threadID + ":test-workflow" },
		func(ctx context.Context, eng workflow.Engine[resumerTestState], d workflow.Definition[resumerTestState], key, _ string) (bool, string, error) {
			_, resumeErr := eng.Resume(ctx, d, key, nil)
			if resumeErr != nil {
				return false, "", resumeErr
			}
			return true, "", nil
		},
	)
	s.Require().NoError(err)

	handled, reply, resumeErr := resumer.Resume(s.ctx, "user-2", "+5511", "sim")
	s.Error(resumeErr)
	s.False(handled)
	s.Empty(reply)
	engine.AssertExpectations(s.T())
}
