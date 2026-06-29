package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"
)

type runtimeDeps struct {
	Token string
	Count int
}

type runtimeState struct {
	Value string `json:"value"`
}

type RuntimeContextSuite struct {
	suite.Suite
	ctx   context.Context
	obs   *fake.Provider
	store *FakeStore
}

func TestRuntimeContextSuite(t *testing.T) {
	suite.Run(t, new(RuntimeContextSuite))
}

func (s *RuntimeContextSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.store = NewFakeStore()
}

func (s *RuntimeContextSuite) TestWithRuntime_StepReceivesValue() {
	deps := &runtimeDeps{Token: "tok-abc", Count: 42}
	ctx := WithRuntime(s.ctx, deps)

	var captured *runtimeDeps
	step := NewStepFunc("capture", func(c context.Context, st runtimeState) (StepOutput[runtimeState], error) {
		rc, ok := RuntimeFrom(c)
		s.True(ok)
		captured, _ = rc.(*runtimeDeps)
		return StepOutput[runtimeState]{State: st, Status: StepStatusCompleted}, nil
	})

	def := Definition[runtimeState]{
		ID:      "rt_workflow",
		Root:    step,
		Durable: false,
	}

	eng := NewEngine[runtimeState](s.store, s.obs)
	result, err := eng.Start(ctx, def, "key:1", runtimeState{Value: "hello"})

	s.NoError(err)
	s.Equal(RunStatusSucceeded, result.Status)
	s.NotNil(captured)
	s.Equal("tok-abc", captured.Token)
	s.Equal(42, captured.Count)
}

func (s *RuntimeContextSuite) TestRuntimeFrom_AbsentWhenNotSet() {
	rc, ok := RuntimeFrom(s.ctx)
	s.False(ok)
	s.Nil(rc)
}

func (s *RuntimeContextSuite) TestRuntimeNotPersistedInSnapshot() {
	deps := &runtimeDeps{Token: "secret-token", Count: 99}
	ctx := WithRuntime(s.ctx, deps)

	suspendStep := NewStepFunc("suspend", func(c context.Context, st runtimeState) (StepOutput[runtimeState], error) {
		return StepOutput[runtimeState]{
			State:  st,
			Status: StepStatusSuspended,
			Suspend: &Suspension{
				Reason: SuspendAwaitingInput,
				Prompt: "waiting for input",
			},
		}, nil
	})

	def := Definition[runtimeState]{
		ID:      "rt_persist_workflow",
		Root:    suspendStep,
		Durable: true,
	}

	eng := NewEngine[runtimeState](s.store, s.obs)
	result, err := eng.Start(ctx, def, "key:suspend", runtimeState{Value: "initial"})

	s.NoError(err)
	s.Equal(RunStatusSuspended, result.Status)

	snap, found, loadErr := s.store.Load(s.ctx, "rt_persist_workflow", "key:suspend")
	s.NoError(loadErr)
	s.True(found)

	var raw map[string]any
	s.NoError(json.Unmarshal(snap.State, &raw))
	s.NotContains(raw, "Token")
	s.NotContains(raw, "Count")
	s.NotContains(raw, "token")
	s.NotContains(raw, "count")
}

func (s *RuntimeContextSuite) TestRuntimeAbsentAfterResumeWithoutReinjection() {
	deps := &runtimeDeps{Token: "ephemeral", Count: 1}
	ctxWithRuntime := WithRuntime(s.ctx, deps)

	var runtimePresentOnResume bool
	callCount := 0

	step := NewStepFunc("check", func(c context.Context, st runtimeState) (StepOutput[runtimeState], error) {
		callCount++
		if callCount == 1 {
			return StepOutput[runtimeState]{
				State:   st,
				Status:  StepStatusSuspended,
				Suspend: &Suspension{Reason: SuspendAwaitingInput},
			}, nil
		}
		_, runtimePresentOnResume = RuntimeFrom(c)
		return StepOutput[runtimeState]{State: st, Status: StepStatusCompleted}, nil
	})

	def := Definition[runtimeState]{
		ID:      "rt_resume_workflow",
		Root:    step,
		Durable: true,
	}

	eng := NewEngine[runtimeState](s.store, s.obs)
	_, startErr := eng.Start(ctxWithRuntime, def, "key:resume", runtimeState{Value: "start"})
	s.NoError(startErr)

	resumePayload := []byte(`{"value":"resumed"}`)
	_, resumeErr := eng.Resume(s.ctx, def, "key:resume", resumePayload)
	s.NoError(resumeErr)

	s.False(runtimePresentOnResume, "runtime context must not be present after resume when not re-injected")
}
