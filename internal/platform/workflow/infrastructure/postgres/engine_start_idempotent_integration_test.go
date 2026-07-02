//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	wfpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type ca04State struct {
	Phase  string `json:"phase"`
	UserID string `json:"user_id"`
}

type EngineStartIdempotentSuite struct {
	suite.Suite
	ctx     context.Context
	factory workflow.StoreFactory
}

func TestEngineStartIdempotentSuite(t *testing.T) {
	suite.Run(t, new(EngineStartIdempotentSuite))
}

func (s *EngineStartIdempotentSuite) SetupSuite() {
	s.ctx = context.Background()
	s.factory = wfpostgres.NewStoreFactory(noop.NewProvider())
}

func (s *EngineStartIdempotentSuite) SetupTest() {}

func (s *EngineStartIdempotentSuite) suspendDef(wfID string) workflow.Definition[ca04State] {
	return workflow.Definition[ca04State]{
		ID: wfID,
		Root: workflow.NewStepFunc("welcome", func(_ context.Context, st ca04State) (workflow.StepOutput[ca04State], error) {
			return workflow.StepOutput[ca04State]{
				State:  ca04State{Phase: "suspended", UserID: st.UserID},
				Status: workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{
					Reason: workflow.SuspendAwaitingInput,
					Prompt: "waiting for user response",
				},
			}, nil
		}),
		Durable: true,
	}
}

func (s *EngineStartIdempotentSuite) TestCA04_ConcurrentStartNoUnexpectedError() {
	db, _ := testcontainer.Postgres(s.T())
	store := s.factory.Store(db)
	eng := workflow.NewEngine[ca04State](store, noop.NewProvider())
	key := uuid.NewString()
	def := s.suspendDef("onboarding-ca04-" + uuid.NewString())
	initial := ca04State{Phase: "welcome", UserID: key}

	const goroutines = 4
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		allErrs   []error
		succeeded atomic.Int32
	)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := eng.Start(s.ctx, def, key, initial)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				allErrs = append(allErrs, err)
			} else {
				succeeded.Add(1)
				s.NotEqual(uuid.Nil, result.RunID, "CA-04: successful Start must return valid RunID")
			}
		}()
	}
	wg.Wait()

	for _, err := range allErrs {
		s.True(
			errors.Is(err, workflow.ErrRunAlreadyExists),
			"CA-04: only ErrRunAlreadyExists is acceptable when a run is already active, got: %v", err,
		)
	}

	s.GreaterOrEqual(int(succeeded.Load()), 1,
		"CA-04: at least 1 goroutine must start the workflow successfully")
}

func (s *EngineStartIdempotentSuite) TestCA04_SequentialStartSecondReturnsRunAlreadyExists() {
	db, _ := testcontainer.Postgres(s.T())
	store := s.factory.Store(db)
	eng := workflow.NewEngine[ca04State](store, noop.NewProvider())
	key := uuid.NewString()
	def := s.suspendDef("onboarding-seq-" + uuid.NewString())
	initial := ca04State{Phase: "welcome", UserID: key}

	r1, err := eng.Start(s.ctx, def, key, initial)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, r1.Status, "first Start must suspend awaiting input")

	_, err = eng.Start(s.ctx, def, key, initial)
	s.True(
		errors.Is(err, workflow.ErrRunAlreadyExists),
		"CA-04: second Start on same key must return ErrRunAlreadyExists (caller must handle as resume)",
	)
}

func (s *EngineStartIdempotentSuite) TestCA04_ResumedOnConflictPathViaConcurrentInsert() {
	db, _ := testcontainer.Postgres(s.T())
	store := s.factory.Store(db)
	eng := workflow.NewEngine[ca04State](store, noop.NewProvider())
	key := uuid.NewString()
	def := s.suspendDef("onboarding-conflict-" + uuid.NewString())
	initial := ca04State{Phase: "welcome", UserID: key}

	snap := workflow.Snapshot{
		RunID:          uuid.New(),
		Workflow:       def.ID,
		CorrelationKey: key,
		Status:         workflow.RunStatusSuspended,
		Cursor:         1,
		State:          []byte(`{"phase":"suspended","user_id":"` + key + `"}`),
		Attempts:       0,
		MaxAttempts:    3,
		Version:        1,
	}
	s.Require().NoError(store.Insert(s.ctx, snap))

	result, err := eng.Start(s.ctx, def, key, initial)
	s.True(
		errors.Is(err, workflow.ErrRunAlreadyExists) || result.RunID == snap.RunID,
		"CA-04: Start on pre-existing suspended run must either return ErrRunAlreadyExists or resume with same RunID",
	)
}
