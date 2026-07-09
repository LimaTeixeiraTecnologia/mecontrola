package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
)

type reaperFakeStore struct {
	suspended []Snapshot
	saved     []Snapshot
	saveErr   error
}

func (f *reaperFakeStore) Insert(_ context.Context, _ Snapshot) error { return nil }
func (f *reaperFakeStore) Load(_ context.Context, _, _ string) (Snapshot, bool, error) {
	return Snapshot{}, false, nil
}
func (f *reaperFakeStore) LoadLatest(_ context.Context, _, _ string) (Snapshot, bool, error) {
	return Snapshot{}, false, nil
}
func (f *reaperFakeStore) Save(_ context.Context, snap Snapshot, _ int64) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, snap)
	return nil
}
func (f *reaperFakeStore) AppendStep(_ context.Context, _ StepRecord) error { return nil }
func (f *reaperFakeStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}
func (f *reaperFakeStore) ListSuspended(_ context.Context, _ string, _ time.Time, _ int) ([]Snapshot, error) {
	return f.suspended, nil
}

type ReaperSuite struct {
	suite.Suite
	ctx context.Context
}

func TestReaperSuite(t *testing.T) {
	suite.Run(t, new(ReaperSuite))
}

func (s *ReaperSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ReaperSuite) TestReap_MarksStaleSuspendedAsFailed() {
	store := &reaperFakeStore{
		suspended: []Snapshot{
			{RunID: uuid.New(), Workflow: "destructive-confirm", Status: RunStatusSuspended, Version: 3},
			{RunID: uuid.New(), Workflow: "destructive-confirm", Status: RunStatusSuspended, Version: 5},
		},
	}
	reaper := NewStaleSuspendedReaper(store, "destructive-confirm", 10*time.Minute, 100, fake.NewProvider())

	count, err := reaper.Reap(s.ctx)

	s.NoError(err)
	s.Equal(int64(2), count)
	s.Len(store.saved, 2)
	for _, snap := range store.saved {
		s.Equal(RunStatusFailed, snap.Status)
		s.NotNil(snap.EndedAt)
		s.NotEmpty(snap.LastError)
	}
}

func (s *ReaperSuite) TestReap_NoSuspended_NoOp() {
	store := &reaperFakeStore{}
	reaper := NewStaleSuspendedReaper(store, "destructive-confirm", 10*time.Minute, 100, fake.NewProvider())

	count, err := reaper.Reap(s.ctx)

	s.NoError(err)
	s.Equal(int64(0), count)
	s.Empty(store.saved)
}

func (s *ReaperSuite) TestReap_SaveConflict_SkipsWithoutError() {
	store := &reaperFakeStore{
		suspended: []Snapshot{{RunID: uuid.New(), Workflow: "destructive-confirm", Status: RunStatusSuspended, Version: 1}},
		saveErr:   ErrVersionConflict,
	}
	reaper := NewStaleSuspendedReaper(store, "destructive-confirm", 10*time.Minute, 100, fake.NewProvider())

	count, err := reaper.Reap(s.ctx)

	s.NoError(err)
	s.Equal(int64(0), count)
}
