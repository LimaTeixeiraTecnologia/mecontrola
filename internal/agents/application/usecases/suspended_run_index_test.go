package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type fakeSuspendedRunStore struct {
	snapshots map[string]workflow.Snapshot
	loadErr   error
}

func newFakeSuspendedRunStore() *fakeSuspendedRunStore {
	return &fakeSuspendedRunStore{snapshots: make(map[string]workflow.Snapshot)}
}

func (f *fakeSuspendedRunStore) put(wf, key string, status workflow.RunStatus) {
	f.snapshots[wf+"|"+key] = workflow.Snapshot{Workflow: wf, CorrelationKey: key, Status: status}
}

func (f *fakeSuspendedRunStore) Insert(_ context.Context, _ workflow.Snapshot) error { return nil }

func (f *fakeSuspendedRunStore) Load(_ context.Context, wf, key string) (workflow.Snapshot, bool, error) {
	if f.loadErr != nil {
		return workflow.Snapshot{}, false, f.loadErr
	}
	snap, ok := f.snapshots[wf+"|"+key]
	return snap, ok, nil
}

func (f *fakeSuspendedRunStore) LoadLatest(_ context.Context, wf, key string) (workflow.Snapshot, bool, error) {
	return f.Load(context.Background(), wf, key)
}

func (f *fakeSuspendedRunStore) Save(_ context.Context, _ workflow.Snapshot, _ int64) error {
	return nil
}

func (f *fakeSuspendedRunStore) AppendStep(_ context.Context, _ workflow.StepRecord) error {
	return nil
}

func (f *fakeSuspendedRunStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

func (f *fakeSuspendedRunStore) ListSuspended(_ context.Context, _ string, _ time.Time, _ int) ([]workflow.Snapshot, error) {
	return nil, nil
}

type SuspendedRunIndexSuite struct {
	suite.Suite
	ctx context.Context
}

func TestSuspendedRunIndexSuite(t *testing.T) {
	suite.Run(t, new(SuspendedRunIndexSuite))
}

func (s *SuspendedRunIndexSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *SuspendedRunIndexSuite) TestResolve() {
	type args struct {
		resourceID string
		threadID   string
	}
	type dependencies struct {
		store *fakeSuspendedRunStore
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(workflowID string, ok bool, err error)
	}{
		{
			name: "deve retornar ok=false quando nenhum workflow esta suspenso",
			args: args{resourceID: "user-1", threadID: "+55110001"},
			dependencies: dependencies{
				store: newFakeSuspendedRunStore(),
			},
			expect: func(workflowID string, ok bool, err error) {
				s.NoError(err)
				s.False(ok)
				s.Empty(workflowID)
			},
		},
		{
			name: "deve resolver o unico workflow suspenso para a thread",
			args: args{resourceID: "user-2", threadID: "+55110002"},
			dependencies: dependencies{
				store: func() *fakeSuspendedRunStore {
					st := newFakeSuspendedRunStore()
					st.put("transaction-write", "user-2:+55110002:transaction-write", workflow.RunStatusSuspended)
					return st
				}(),
			},
			expect: func(workflowID string, ok bool, err error) {
				s.NoError(err)
				s.True(ok)
				s.Equal("transaction-write", workflowID)
			},
		},
		{
			name: "deve ignorar workflow concluido e resolver apenas o suspenso",
			args: args{resourceID: "user-3", threadID: "+55110003"},
			dependencies: dependencies{
				store: func() *fakeSuspendedRunStore {
					st := newFakeSuspendedRunStore()
					st.put("transaction-write", "user-3:+55110003:transaction-write", workflow.RunStatusSucceeded)
					st.put("card-manage", "user-3:+55110003:card-manage", workflow.RunStatusSuspended)
					return st
				}(),
			},
			expect: func(workflowID string, ok bool, err error) {
				s.NoError(err)
				s.True(ok)
				s.Equal("card-manage", workflowID)
			},
		},
		{
			name: "deve retornar erro quando store falha",
			args: args{resourceID: "user-4", threadID: "+55110004"},
			dependencies: dependencies{
				store: func() *fakeSuspendedRunStore {
					st := newFakeSuspendedRunStore()
					st.loadErr = errors.New("db indisponivel")
					return st
				}(),
			},
			expect: func(workflowID string, ok bool, err error) {
				s.Error(err)
				s.False(ok)
				s.Empty(workflowID)
			},
		},
		{
			name: "deve retornar ErrMultipleSuspendedRuns quando mais de um workflow esta suspenso na mesma thread",
			args: args{resourceID: "user-5", threadID: "+55110005"},
			dependencies: dependencies{
				store: func() *fakeSuspendedRunStore {
					st := newFakeSuspendedRunStore()
					st.put("transaction-write", "user-5:+55110005:transaction-write", workflow.RunStatusSuspended)
					st.put("budget-manage", "user-5:+55110005:budget-manage", workflow.RunStatusSuspended)
					return st
				}(),
			},
			expect: func(workflowID string, ok bool, err error) {
				s.Error(err)
				s.True(errors.Is(err, ErrMultipleSuspendedRuns))
				s.False(ok)
				s.Empty(workflowID)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			idx := NewSuspendedRunIndex(scenario.dependencies.store, "transaction-write", "budget-manage", "card-manage", "goal-edit", "destructive-confirm")
			workflowID, ok, err := idx.Resolve(s.ctx, scenario.args.resourceID, scenario.args.threadID)
			scenario.expect(workflowID, ok, err)
		})
	}
}
