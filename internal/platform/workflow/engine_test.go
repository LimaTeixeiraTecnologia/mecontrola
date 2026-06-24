package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type engineTestState struct {
	Value int `json:"value"`
}

type EngineTestSuite struct {
	suite.Suite
	ctx   context.Context
	obs   *fake.Provider
	store *FakeStore
}

func TestEngineTestSuite(t *testing.T) {
	suite.Run(t, new(EngineTestSuite))
}

func (s *EngineTestSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.store = NewFakeStore()
}

func makeEngineStep(id string, delta int) Step[engineTestState] {
	return NewStepFunc(id, func(_ context.Context, st engineTestState) (StepOutput[engineTestState], error) {
		return StepOutput[engineTestState]{State: engineTestState{Value: st.Value + delta}, Status: StepStatusCompleted}, nil
	})
}

func makeSuspendEngineStep(id string) Step[engineTestState] {
	return NewStepFunc(id, func(_ context.Context, st engineTestState) (StepOutput[engineTestState], error) {
		return StepOutput[engineTestState]{
			State:   st,
			Status:  StepStatusSuspended,
			Suspend: &Suspension{Reason: SuspendAwaitingInput, Prompt: "waiting"},
		}, nil
	})
}

func makeErrorEngineStep(id string, err error) Step[engineTestState] {
	return NewStepFunc(id, func(_ context.Context, st engineTestState) (StepOutput[engineTestState], error) {
		return StepOutput[engineTestState]{State: st}, err
	})
}

func makeFailStatusEngineStep(id string) Step[engineTestState] {
	return NewStepFunc(id, func(_ context.Context, st engineTestState) (StepOutput[engineTestState], error) {
		return StepOutput[engineTestState]{State: st, Status: StepStatusFailed}, nil
	})
}

func (s *EngineTestSuite) TestStart_AutoComplete_Durable() {
	def := Definition[engineTestState]{
		ID:          "test_workflow",
		Root:        Sequence[engineTestState]("root", makeEngineStep("a", 1), makeEngineStep("b", 10)),
		Durable:     true,
		MaxAttempts: 3,
	}

	eng := NewEngine[engineTestState](s.store, s.obs)
	result, err := eng.Start(s.ctx, def, "user:ch", engineTestState{Value: 0})

	s.NoError(err)
	s.Equal(RunStatusSucceeded, result.Status)
	s.Equal(11, result.State.Value)
	s.NotEqual(uuid.Nil, result.RunID)

	snap, found, loadErr := s.store.Load(s.ctx, "test_workflow", "user:ch")
	s.NoError(loadErr)
	s.True(found)
	s.Equal(RunStatusSucceeded, snap.Status)
}

func (s *EngineTestSuite) TestStart_AutoComplete_NotDurable() {
	def := Definition[engineTestState]{
		ID:          "read_workflow",
		Root:        Sequence[engineTestState]("root", makeEngineStep("a", 5)),
		Durable:     false,
		MaxAttempts: 1,
	}

	eng := NewEngine[engineTestState](s.store, s.obs)
	result, err := eng.Start(s.ctx, def, "user:ch", engineTestState{Value: 0})

	s.NoError(err)
	s.Equal(RunStatusSucceeded, result.Status)
	s.Equal(5, result.State.Value)

	_, found, _ := s.store.Load(s.ctx, "read_workflow", "user:ch")
	s.False(found)
}

func (s *EngineTestSuite) TestStart_Suspend_Durable() {
	def := Definition[engineTestState]{
		ID: "suspend_workflow",
		Root: Sequence[engineTestState]("root",
			makeEngineStep("a", 1),
			makeSuspendEngineStep("suspend"),
			makeEngineStep("c", 100),
		),
		Durable:     true,
		MaxAttempts: 3,
	}

	eng := NewEngine[engineTestState](s.store, s.obs)
	result, err := eng.Start(s.ctx, def, "user:ch", engineTestState{Value: 0})

	s.NoError(err)
	s.Equal(RunStatusSuspended, result.Status)
	s.Equal(1, result.State.Value)
	s.NotNil(result.Suspend)
	s.Equal(SuspendAwaitingInput, result.Suspend.Reason)

	snap, found, _ := s.store.Load(s.ctx, "suspend_workflow", "user:ch")
	s.True(found)
	s.Equal(RunStatusSuspended, snap.Status)
	s.Equal(1, snap.Cursor)
}

func (s *EngineTestSuite) TestResume_AfterSuspend() {
	callCount := 0
	resumableStep := NewStepFunc("resumable", func(_ context.Context, st engineTestState) (StepOutput[engineTestState], error) {
		callCount++
		if callCount == 1 {
			return StepOutput[engineTestState]{
				State:   st,
				Status:  StepStatusSuspended,
				Suspend: &Suspension{Reason: SuspendAwaitingInput, Prompt: "waiting"},
			}, nil
		}
		return StepOutput[engineTestState]{State: engineTestState{Value: st.Value + 10}, Status: StepStatusCompleted}, nil
	})

	def := Definition[engineTestState]{
		ID: "resume_workflow",
		Root: Sequence[engineTestState]("root",
			makeEngineStep("a", 1),
			resumableStep,
			makeEngineStep("c", 100),
		),
		Durable:     true,
		MaxAttempts: 3,
	}

	eng := NewEngine[engineTestState](s.store, s.obs)
	startResult, err := eng.Start(s.ctx, def, "user:ch", engineTestState{Value: 0})
	s.NoError(err)
	s.Equal(RunStatusSuspended, startResult.Status)

	resumeResult, err := eng.Resume(s.ctx, def, "user:ch", nil)

	s.NoError(err)
	s.Equal(RunStatusSucceeded, resumeResult.Status)
	s.Equal(111, resumeResult.State.Value)

	snap, found, _ := s.store.Load(s.ctx, "resume_workflow", "user:ch")
	s.True(found)
	s.Equal(RunStatusSucceeded, snap.Status)
}

func (s *EngineTestSuite) TestResume_NotFound_ReturnsEmpty() {
	def := Definition[engineTestState]{
		ID:          "missing_workflow",
		Root:        Sequence[engineTestState]("root", makeEngineStep("a", 1)),
		Durable:     true,
		MaxAttempts: 1,
	}

	eng := NewEngine[engineTestState](s.store, s.obs)
	result, err := eng.Resume(s.ctx, def, "no:key", nil)

	s.NoError(err)
	s.Equal(RunStatus(0), result.Status)
}

func (s *EngineTestSuite) TestResume_NotDurable_ReturnsEmpty() {
	def := Definition[engineTestState]{
		ID:          "nd_workflow",
		Root:        Sequence[engineTestState]("root", makeEngineStep("a", 1)),
		Durable:     false,
		MaxAttempts: 1,
	}

	eng := NewEngine[engineTestState](s.store, s.obs)
	result, err := eng.Resume(s.ctx, def, "user:ch", nil)

	s.NoError(err)
	s.Equal(RunStatus(0), result.Status)
}

func (s *EngineTestSuite) TestStart_TerminalFailure_OnStepError() {
	def := Definition[engineTestState]{
		ID: "fail_workflow",
		Root: Sequence[engineTestState]("root",
			makeEngineStep("a", 1),
			makeErrorEngineStep("boom", errors.New("infra error")),
			makeEngineStep("c", 100),
		),
		Durable:     true,
		MaxAttempts: 3,
	}

	eng := NewEngine[engineTestState](s.store, s.obs)
	result, err := eng.Start(s.ctx, def, "user:ch", engineTestState{Value: 0})

	s.Error(err)
	s.Equal(RunStatusFailed, result.Status)

	snap, found, _ := s.store.Load(s.ctx, "fail_workflow", "user:ch")
	s.True(found)
	s.Equal(RunStatusFailed, snap.Status)
	s.NotEmpty(snap.LastError)
}

func (s *EngineTestSuite) TestStart_TerminalFailure_OnStepStatusFailed() {
	def := Definition[engineTestState]{
		ID: "fail_status_workflow",
		Root: Sequence[engineTestState]("root",
			makeEngineStep("a", 1),
			makeFailStatusEngineStep("guard"),
			makeEngineStep("c", 100),
		),
		Durable:     true,
		MaxAttempts: 3,
	}

	eng := NewEngine[engineTestState](s.store, s.obs)
	result, err := eng.Start(s.ctx, def, "user:ch", engineTestState{Value: 0})

	s.Error(err)
	s.Equal(RunStatusFailed, result.Status)
}

func (s *EngineTestSuite) TestStart_ShortCircuit_Durable_StoreHasStepRecords() {
	def := Definition[engineTestState]{
		ID: "short_circuit_workflow",
		Root: Sequence[engineTestState]("root",
			makeEngineStep("a", 1),
			makeEngineStep("b", 10),
		),
		Durable:     true,
		MaxAttempts: 1,
	}

	eng := NewEngine[engineTestState](s.store, s.obs)
	result, err := eng.Start(s.ctx, def, "user:ch", engineTestState{Value: 0})

	s.NoError(err)
	s.Equal(RunStatusSucceeded, result.Status)
	s.Len(s.store.Steps(), 2)
}

func (s *EngineTestSuite) TestMetrics_EmittedOnStart() {
	def := Definition[engineTestState]{
		ID:          "metrics_workflow",
		Root:        Sequence[engineTestState]("root", makeEngineStep("a", 1)),
		Durable:     false,
		MaxAttempts: 1,
	}

	eng := NewEngine[engineTestState](s.store, s.obs)
	_, err := eng.Start(s.ctx, def, "user:ch", engineTestState{Value: 0})
	s.NoError(err)

	fakeMetrics := s.obs.Metrics().(*fake.FakeMetrics)
	runsTotal := fakeMetrics.GetCounter("workflow_runs_total")
	s.NotNil(runsTotal)
	s.NotEmpty(runsTotal.GetValues())

	stepsTotal := fakeMetrics.GetCounter("workflow_steps_total")
	s.NotNil(stepsTotal)
	s.NotEmpty(stepsTotal.GetValues())
}

func (s *EngineTestSuite) TestCursorResume_SkipsCompletedSteps() {
	callCount := 0
	countingStep := NewStepFunc("counting", func(_ context.Context, st engineTestState) (StepOutput[engineTestState], error) {
		callCount++
		return StepOutput[engineTestState]{State: engineTestState{Value: st.Value + 1}, Status: StepStatusCompleted}, nil
	})

	suspendCallCount := 0
	resumableStep := NewStepFunc("suspend", func(_ context.Context, st engineTestState) (StepOutput[engineTestState], error) {
		suspendCallCount++
		if suspendCallCount == 1 {
			return StepOutput[engineTestState]{State: st, Status: StepStatusSuspended, Suspend: &Suspension{Reason: SuspendAwaitingInput}}, nil
		}
		return StepOutput[engineTestState]{State: engineTestState{Value: st.Value + 10}, Status: StepStatusCompleted}, nil
	})

	def := Definition[engineTestState]{
		ID: "cursor_workflow",
		Root: Sequence[engineTestState]("root",
			countingStep,
			resumableStep,
			countingStep,
		),
		Durable:     true,
		MaxAttempts: 1,
	}

	eng := NewEngine[engineTestState](s.store, s.obs)
	_, _ = eng.Start(s.ctx, def, "user:ch", engineTestState{Value: 0})

	startCalls := callCount

	_, _ = eng.Resume(s.ctx, def, "user:ch", nil)

	s.Equal(startCalls+1, callCount)
}

func (s *EngineTestSuite) TestVersionConflict_TriggersMetric() {
	def := Definition[engineTestState]{
		ID:          "conflict_workflow",
		Root:        Sequence[engineTestState]("root", makeEngineStep("a", 1)),
		Durable:     true,
		MaxAttempts: 1,
	}

	s.store.SetSaveError(ErrVersionConflict)

	eng := NewEngine[engineTestState](s.store, s.obs)
	_, _ = eng.Start(s.ctx, def, "user:ch", engineTestState{Value: 0})

	fakeMetrics := s.obs.Metrics().(*fake.FakeMetrics)
	conflictCounter := fakeMetrics.GetCounter("workflow_version_conflict_total")
	s.NotNil(conflictCounter)
}

type FakeStore struct {
	mu        sync.RWMutex
	snaps     map[string]Snapshot
	steps     []StepRecord
	saveErr   error
	insertErr error
}

func NewFakeStore() *FakeStore {
	return &FakeStore{
		snaps: make(map[string]Snapshot),
	}
}

func (f *FakeStore) SetSaveError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveErr = err
}

func (f *FakeStore) SetInsertError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.insertErr = err
}

func (f *FakeStore) Steps() []StepRecord {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]StepRecord, len(f.steps))
	copy(out, f.steps)
	return out
}

func (f *FakeStore) storeKey(workflow, correlationKey string) string {
	return workflow + "::" + correlationKey
}

func (f *FakeStore) Insert(_ context.Context, snap Snapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.insertErr != nil {
		return f.insertErr
	}
	f.snaps[f.storeKey(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (f *FakeStore) Load(_ context.Context, workflow, key string) (Snapshot, bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	snap, ok := f.snaps[f.storeKey(workflow, key)]
	if !ok {
		return Snapshot{}, false, nil
	}
	return snap, true, nil
}

func (f *FakeStore) Save(_ context.Context, snap Snapshot, expectedVersion int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	existing, ok := f.snaps[f.storeKey(snap.Workflow, snap.CorrelationKey)]
	if ok && existing.Version != expectedVersion {
		return ErrVersionConflict
	}
	f.snaps[f.storeKey(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (f *FakeStore) AppendStep(_ context.Context, rec StepRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.steps = append(f.steps, rec)
	return nil
}

func (f *FakeStore) DeleteCompleted(_ context.Context, retention time.Duration, limit int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cutoff := time.Now().UTC().Add(-retention)
	var deleted int64
	for k, snap := range f.snaps {
		if snap.Status == RunStatusSucceeded || snap.Status == RunStatusFailed {
			if snap.EndedAt != nil && snap.EndedAt.Before(cutoff) {
				if limit > 0 && int(deleted) >= limit {
					break
				}
				delete(f.snaps, k)
				deleted++
			}
		}
	}
	return deleted, nil
}

func (s *EngineTestSuite) TestFakeStore_Insert_Load() {
	snap := Snapshot{
		RunID:          uuid.New(),
		Workflow:       "wf",
		CorrelationKey: "k",
		Status:         RunStatusRunning,
		Version:        1,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	s.NoError(s.store.Insert(s.ctx, snap))

	loaded, found, err := s.store.Load(s.ctx, "wf", "k")
	s.NoError(err)
	s.True(found)
	s.Equal(snap.RunID, loaded.RunID)
}

func (s *EngineTestSuite) TestFakeStore_Save_CAS() {
	snap := Snapshot{
		RunID:          uuid.New(),
		Workflow:       "wf",
		CorrelationKey: "k",
		Status:         RunStatusRunning,
		Version:        1,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	s.NoError(s.store.Insert(s.ctx, snap))

	snap.Status = RunStatusSucceeded
	snap.Version = 2
	s.NoError(s.store.Save(s.ctx, snap, 1))

	wrongVersion := snap
	wrongVersion.Status = RunStatusFailed
	wrongVersion.Version = 5
	err := s.store.Save(s.ctx, wrongVersion, 99)
	s.ErrorIs(err, ErrVersionConflict)
}

func (s *EngineTestSuite) TestFakeStore_DeleteCompleted_Retention() {
	old := time.Now().UTC().Add(-48 * time.Hour)
	oldSnap := Snapshot{
		RunID:          uuid.New(),
		Workflow:       "wf",
		CorrelationKey: "old",
		Status:         RunStatusSucceeded,
		Version:        1,
		CreatedAt:      old,
		UpdatedAt:      old,
		EndedAt:        &old,
	}
	s.store.snaps[s.store.storeKey("wf", "old")] = oldSnap

	recentEnd := time.Now().UTC().Add(-1 * time.Hour)
	recent := Snapshot{
		RunID:          uuid.New(),
		Workflow:       "wf",
		CorrelationKey: "recent",
		Status:         RunStatusSucceeded,
		Version:        1,
		CreatedAt:      recentEnd,
		UpdatedAt:      recentEnd,
		EndedAt:        &recentEnd,
	}
	s.store.snaps[s.store.storeKey("wf", "recent")] = recent

	deleted, err := s.store.DeleteCompleted(s.ctx, 24*time.Hour, 100)
	s.NoError(err)
	s.Equal(int64(1), deleted)

	_, foundOld, _ := s.store.Load(s.ctx, "wf", "old")
	_, foundRecent, _ := s.store.Load(s.ctx, "wf", "recent")
	s.False(foundOld)
	s.True(foundRecent)
}
