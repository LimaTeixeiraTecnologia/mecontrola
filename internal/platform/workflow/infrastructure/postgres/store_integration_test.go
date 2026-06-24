//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	wfpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type StoreIntegrationSuite struct {
	suite.Suite
	ctx     context.Context
	db      *sqlx.DB
	factory workflow.StoreFactory
}

func TestStoreIntegrationSuite(t *testing.T) {
	suite.Run(t, new(StoreIntegrationSuite))
}

func (s *StoreIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
	s.factory = wfpostgres.NewStoreFactory(noop.NewProvider())
}

func (s *StoreIntegrationSuite) SetupTest() {}

func (s *StoreIntegrationSuite) store() workflow.Store {
	return s.factory.Store(s.db)
}

func (s *StoreIntegrationSuite) newSnap(wf, key string) workflow.Snapshot {
	return workflow.Snapshot{
		RunID:          uuid.New(),
		Workflow:       wf,
		CorrelationKey: key,
		Status:         workflow.RunStatusRunning,
		Cursor:         0,
		State:          []byte(`{"value":1}`),
		Attempts:       0,
		MaxAttempts:    3,
		Version:        1,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
}

func (s *StoreIntegrationSuite) TestInsertAndLoad() {
	store := s.store()
	snap := s.newSnap("test_wf", uuid.NewString())

	err := store.Insert(s.ctx, snap)
	s.Require().NoError(err)

	loaded, found, err := store.Load(s.ctx, "test_wf", snap.CorrelationKey)
	s.Require().NoError(err)
	s.True(found)
	s.Equal(snap.RunID, loaded.RunID)
	s.Equal(snap.Workflow, loaded.Workflow)
	s.Equal(snap.CorrelationKey, loaded.CorrelationKey)
	s.Equal(workflow.RunStatusRunning, loaded.Status)
	s.Equal(int64(1), loaded.Version)
}

func (s *StoreIntegrationSuite) TestSaveCASSuccess() {
	store := s.store()
	snap := s.newSnap("test_wf_cas", uuid.NewString())

	err := store.Insert(s.ctx, snap)
	s.Require().NoError(err)

	snap.Status = workflow.RunStatusSuspended
	snap.Cursor = 1
	snap.UpdatedAt = time.Now().UTC()

	err = store.Save(s.ctx, snap, 1)
	s.Require().NoError(err)

	loaded, found, err := store.Load(s.ctx, snap.Workflow, snap.CorrelationKey)
	s.Require().NoError(err)
	s.True(found)
	s.Equal(workflow.RunStatusSuspended, loaded.Status)
	s.Equal(1, loaded.Cursor)
	s.Equal(int64(2), loaded.Version)
}

func (s *StoreIntegrationSuite) TestSaveCASVersionConflict() {
	store := s.store()
	snap := s.newSnap("test_wf_conflict", uuid.NewString())

	err := store.Insert(s.ctx, snap)
	s.Require().NoError(err)

	snap.Status = workflow.RunStatusSuspended
	snap.UpdatedAt = time.Now().UTC()

	err = store.Save(s.ctx, snap, 999)
	s.Require().Error(err)
	s.ErrorIs(err, workflow.ErrVersionConflict)
}

func (s *StoreIntegrationSuite) TestLoadNotFound() {
	store := s.store()
	_, found, err := store.Load(s.ctx, "nonexistent", "nonexistent-key")
	s.Require().NoError(err)
	s.False(found)
}

func (s *StoreIntegrationSuite) TestDurabilityResume() {
	store := s.store()
	key := uuid.NewString()
	snap := s.newSnap("durable_wf", key)

	err := store.Insert(s.ctx, snap)
	s.Require().NoError(err)

	snap.Status = workflow.RunStatusSuspended
	snap.Cursor = 2
	snap.SuspendReason = workflow.SuspendAwaitingInput
	snap.State = []byte(`{"value":42}`)
	snap.UpdatedAt = time.Now().UTC()

	err = store.Save(s.ctx, snap, 1)
	s.Require().NoError(err)

	store2 := s.factory.Store(s.db)
	loaded, found, err := store2.Load(s.ctx, "durable_wf", key)
	s.Require().NoError(err)
	s.True(found)
	s.Equal(workflow.RunStatusSuspended, loaded.Status)
	s.Equal(2, loaded.Cursor)
	s.Equal(workflow.SuspendAwaitingInput, loaded.SuspendReason)
	s.JSONEq(`{"value":42}`, string(loaded.State))
	s.Equal(int64(2), loaded.Version)

	loaded.Status = workflow.RunStatusSucceeded
	now := time.Now().UTC()
	loaded.EndedAt = &now
	loaded.UpdatedAt = now

	err = store2.Save(s.ctx, loaded, 2)
	s.Require().NoError(err)

	_, found2, err := store2.Load(s.ctx, "durable_wf", key)
	s.Require().NoError(err)
	s.False(found2)
}

func (s *StoreIntegrationSuite) TestConcurrentResumeCASWinner() {
	store := s.store()
	key := uuid.NewString()
	snap := s.newSnap("concurrent_wf", key)
	snap.Status = workflow.RunStatusSuspended
	snap.Cursor = 1

	err := store.Insert(s.ctx, snap)
	s.Require().NoError(err)

	snap.Status = workflow.RunStatusSuspended
	snap.UpdatedAt = time.Now().UTC()
	err = store.Save(s.ctx, snap, 1)
	s.Require().NoError(err)

	loaded, found, err := store.Load(s.ctx, "concurrent_wf", key)
	s.Require().NoError(err)
	s.True(found)

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		wins   int
		losses int
	)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s2 := s.factory.Store(s.db)
			snap2 := loaded
			snap2.Status = workflow.RunStatusSucceeded
			now := time.Now().UTC()
			snap2.EndedAt = &now
			snap2.UpdatedAt = now

			saveErr := s2.Save(context.Background(), snap2, loaded.Version)
			mu.Lock()
			defer mu.Unlock()
			if saveErr == nil {
				wins++
			} else {
				losses++
			}
		}()
	}

	wg.Wait()

	s.Equal(1, wins)
	s.Equal(1, losses)
}

func (s *StoreIntegrationSuite) TestPartialIndexPreventsDoubleActiveRun() {
	store := s.store()
	key := uuid.NewString()
	snap1 := s.newSnap("dup_active_wf", key)

	err := store.Insert(s.ctx, snap1)
	s.Require().NoError(err)

	snap2 := s.newSnap("dup_active_wf", key)
	err = store.Insert(s.ctx, snap2)
	s.Require().Error(err)
}

func (s *StoreIntegrationSuite) TestAppendStep() {
	store := s.store()
	snap := s.newSnap("step_wf", uuid.NewString())

	err := store.Insert(s.ctx, snap)
	s.Require().NoError(err)

	now := time.Now().UTC()
	rec := workflow.StepRecord{
		ID:         uuid.New(),
		RunID:      snap.RunID,
		StepID:     "step_one",
		Seq:        0,
		Status:     workflow.StepStatusCompleted,
		Attempt:    1,
		DurationMs: 42,
		Error:      "",
		StartedAt:  now,
		EndedAt:    &now,
	}

	err = store.AppendStep(s.ctx, rec)
	s.Require().NoError(err)
}

func (s *StoreIntegrationSuite) TestDeleteCompleted() {
	store := s.store()
	key := uuid.NewString()
	snap := s.newSnap("gc_wf", key)

	err := store.Insert(s.ctx, snap)
	s.Require().NoError(err)

	snap.Status = workflow.RunStatusSucceeded
	past := time.Now().UTC().Add(-48 * time.Hour)
	snap.UpdatedAt = past
	snap.EndedAt = &past

	err = store.Save(s.ctx, snap, 1)
	s.Require().NoError(err)

	_, err = s.db.ExecContext(s.ctx,
		`UPDATE mecontrola.workflow_runs SET updated_at = $1 WHERE id = $2`,
		past, snap.RunID,
	)
	s.Require().NoError(err)

	deleted, err := store.DeleteCompleted(s.ctx, 24*time.Hour, 100)
	s.Require().NoError(err)
	s.GreaterOrEqual(deleted, int64(1))
}
