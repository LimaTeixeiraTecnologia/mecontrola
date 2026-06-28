package handlers

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type abandonmentFakeStore struct {
	mu    sync.RWMutex
	snaps map[uuid.UUID]platform.Snapshot
}

func newAbandonmentFakeStore() *abandonmentFakeStore {
	return &abandonmentFakeStore{snaps: make(map[uuid.UUID]platform.Snapshot)}
}

func (f *abandonmentFakeStore) Insert(_ context.Context, snap platform.Snapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snaps[snap.RunID] = snap
	return nil
}

func (f *abandonmentFakeStore) Load(_ context.Context, _, _ string) (platform.Snapshot, bool, error) {
	return platform.Snapshot{}, false, nil
}

func (f *abandonmentFakeStore) Save(_ context.Context, snap platform.Snapshot, expectedVersion int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.snaps[snap.RunID]
	if ok && existing.Version != expectedVersion {
		return platform.ErrVersionConflict
	}
	snap.Version++
	f.snaps[snap.RunID] = snap
	return nil
}

func (f *abandonmentFakeStore) AppendStep(_ context.Context, _ platform.StepRecord) error { return nil }
func (f *abandonmentFakeStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

func (f *abandonmentFakeStore) ListSuspended(_ context.Context, wf string, updatedBefore time.Time, limit int) ([]platform.Snapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var result []platform.Snapshot
	for _, snap := range f.snaps {
		if snap.Workflow != wf {
			continue
		}
		if snap.Status != platform.RunStatusSuspended {
			continue
		}
		if !snap.UpdatedAt.Before(updatedBefore) {
			continue
		}
		result = append(result, snap)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

type abandonmentFakeFactory struct {
	store *abandonmentFakeStore
}

func (f *abandonmentFakeFactory) Store(_ database.DBTX) platform.Store { return f.store }

type abandonmentFakeUoW struct{ store *abandonmentFakeStore }

func (u *abandonmentFakeUoW) DBTX() database.DBTX { return nil }
func (u *abandonmentFakeUoW) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

type OnboardingAbandonmentJobSuite struct {
	suite.Suite
	ctx   context.Context
	obs   *fake.Provider
	store *abandonmentFakeStore
}

func TestOnboardingAbandonmentJobSuite(t *testing.T) {
	suite.Run(t, new(OnboardingAbandonmentJobSuite))
}

func (s *OnboardingAbandonmentJobSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.store = newAbandonmentFakeStore()
}

func (s *OnboardingAbandonmentJobSuite) newJob() *OnboardingAbandonmentJob {
	cfg := configs.OnboardingConfig{
		AbandonmentTTLHours:    48,
		AbandonmentJobSchedule: "@hourly",
		AbandonmentBatchSize:   10,
	}
	job, err := NewOnboardingAbandonmentJob(
		&abandonmentFakeUoW{store: s.store},
		&abandonmentFakeFactory{store: s.store},
		cfg,
		s.obs,
	)
	s.Require().NoError(err)
	return job
}

func (s *OnboardingAbandonmentJobSuite) suspendedSnap(phase valueobjects.OnboardingPhase, updatedAt time.Time, abandonedAt time.Time) platform.Snapshot {
	state := workflow.OnboardingState{
		UserID:      uuid.New(),
		Phase:       phase,
		SuspendedAt: updatedAt,
		AbandonedAt: abandonedAt,
	}
	stateBytes, _ := json.Marshal(state)
	return platform.Snapshot{
		RunID:          uuid.New(),
		Workflow:       onboardingWorkflowID,
		CorrelationKey: uuid.New().String(),
		Status:         platform.RunStatusSuspended,
		State:          stateBytes,
		Version:        1,
		UpdatedAt:      updatedAt,
	}
}

func (s *OnboardingAbandonmentJobSuite) TestRun_SelectsInactiveAndEmitsMetric() {
	job := s.newJob()
	old := s.suspendedSnap(valueobjects.PhaseBudget, time.Now().UTC().Add(-72*time.Hour), time.Time{})
	recent := s.suspendedSnap(valueobjects.PhaseValues, time.Now().UTC().Add(-1*time.Hour), time.Time{})
	s.Require().NoError(s.store.Insert(s.ctx, old))
	s.Require().NoError(s.store.Insert(s.ctx, recent))

	err := job.Run(s.ctx)
	s.NoError(err)

	counter := s.obs.Metrics().(*fake.FakeMetrics).GetCounter("onboarding_step_abandoned_total")
	s.Require().NotNil(counter)
	values := counter.GetValues()
	s.Len(values, 1)
	s.Equal(int64(1), values[0].Value)
	s.True(hasLabel(values[0].Fields, "step", "budget"))

	saved := s.store.snaps[old.RunID]
	s.False(saved.State == nil)
	var savedState workflow.OnboardingState
	s.NoError(json.Unmarshal(saved.State, &savedState))
	s.False(savedState.AbandonedAt.IsZero())
}

func (s *OnboardingAbandonmentJobSuite) TestRun_IsIdempotent() {
	job := s.newJob()
	old := s.suspendedSnap(valueobjects.PhaseCards, time.Now().UTC().Add(-72*time.Hour), time.Time{})
	s.Require().NoError(s.store.Insert(s.ctx, old))

	s.NoError(job.Run(s.ctx))
	s.NoError(job.Run(s.ctx))

	counter := s.obs.Metrics().(*fake.FakeMetrics).GetCounter("onboarding_step_abandoned_total")
	s.Require().NotNil(counter)
	s.Len(counter.GetValues(), 1)
}

func (s *OnboardingAbandonmentJobSuite) TestRun_SkipsOtherWorkflows() {
	job := s.newJob()
	state := workflow.OnboardingState{UserID: uuid.New(), Phase: valueobjects.PhaseObjective}
	stateBytes, _ := json.Marshal(state)
	other := platform.Snapshot{
		RunID:          uuid.New(),
		Workflow:       "other",
		CorrelationKey: uuid.New().String(),
		Status:         platform.RunStatusSuspended,
		State:          stateBytes,
		Version:        1,
		UpdatedAt:      time.Now().UTC().Add(-72 * time.Hour),
	}
	s.Require().NoError(s.store.Insert(s.ctx, other))

	s.NoError(job.Run(s.ctx))

	counter := s.obs.Metrics().(*fake.FakeMetrics).GetCounter("onboarding_step_abandoned_total")
	s.Require().NotNil(counter)
	s.Empty(counter.GetValues())
}

func (s *OnboardingAbandonmentJobSuite) TestNewJob_ValidatesConfig() {
	_, err := NewOnboardingAbandonmentJob(
		&abandonmentFakeUoW{store: s.store},
		&abandonmentFakeFactory{store: s.store},
		configs.OnboardingConfig{AbandonmentTTLHours: 0},
		s.obs,
	)
	s.Error(err)
}

func hasLabel(fields []observability.Field, key, value string) bool {
	for _, f := range fields {
		if f.Key == key && f.StringValue() == value {
			return true
		}
	}
	return false
}
