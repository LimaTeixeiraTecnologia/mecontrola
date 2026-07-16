package workflows

import (
	"context"
	"sync"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type wfStore struct {
	mu   sync.RWMutex
	data map[string]workflow.Snapshot
}

func newWfStore() *wfStore {
	return &wfStore{data: make(map[string]workflow.Snapshot)}
}

func (s *wfStore) key(wid, ck string) string { return wid + "::" + ck }

func (s *wfStore) Insert(_ context.Context, snap workflow.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(snap.Workflow, snap.CorrelationKey)
	if ex, ok := s.data[k]; ok {
		if ex.Status == workflow.RunStatusRunning || ex.Status == workflow.RunStatusSuspended {
			return workflow.ErrRunAlreadyExists
		}
	}
	s.data[k] = snap
	return nil
}

func (s *wfStore) Load(_ context.Context, wid, key string) (workflow.Snapshot, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.data[s.key(wid, key)]
	return snap, ok, nil
}

func (s *wfStore) LoadLatest(_ context.Context, wid, key string) (workflow.Snapshot, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.data[s.key(wid, key)]
	return snap, ok, nil
}

func (s *wfStore) Save(_ context.Context, snap workflow.Snapshot, expected int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(snap.Workflow, snap.CorrelationKey)
	if ex, ok := s.data[k]; ok && ex.Version != expected {
		return workflow.ErrVersionConflict
	}
	s.data[k] = snap
	return nil
}

func (s *wfStore) AppendStep(_ context.Context, _ workflow.StepRecord) error { return nil }

func (s *wfStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

func (s *wfStore) ListSuspended(_ context.Context, _ string, _ time.Time, _ int) ([]workflow.Snapshot, error) {
	return nil, nil
}
