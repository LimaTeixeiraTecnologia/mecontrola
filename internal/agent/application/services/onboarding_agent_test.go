package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type onboardingAgentFakeStore struct {
	snaps map[string]platform.Snapshot
}

func newOnboardingAgentFakeStore() *onboardingAgentFakeStore {
	return &onboardingAgentFakeStore{snaps: make(map[string]platform.Snapshot)}
}

func (f *onboardingAgentFakeStore) Insert(_ context.Context, snap platform.Snapshot) error {
	f.snaps[f.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (f *onboardingAgentFakeStore) Load(_ context.Context, wf, key string) (platform.Snapshot, bool, error) {
	snap, ok := f.snaps[f.key(wf, key)]
	return snap, ok, nil
}

func (f *onboardingAgentFakeStore) Save(_ context.Context, snap platform.Snapshot, expectedVersion int64) error {
	existing, ok := f.snaps[f.key(snap.Workflow, snap.CorrelationKey)]
	if ok && existing.Version != expectedVersion {
		return platform.ErrVersionConflict
	}
	snap.Version++
	f.snaps[f.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (f *onboardingAgentFakeStore) AppendStep(_ context.Context, _ platform.StepRecord) error {
	return nil
}
func (f *onboardingAgentFakeStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}
func (f *onboardingAgentFakeStore) ListSuspended(_ context.Context, _ string, _ time.Time, _ int) ([]platform.Snapshot, error) {
	return nil, nil
}
func (f *onboardingAgentFakeStore) key(wf, corr string) string { return wf + "::" + corr }

type onboardingAgentFakeEngine struct {
	result platform.RunResult[workflow.OnboardingState]
	err    error
}

func (e *onboardingAgentFakeEngine) Start(_ context.Context, _ platform.Definition[workflow.OnboardingState], _ string, initial workflow.OnboardingState) (platform.RunResult[workflow.OnboardingState], error) {
	return e.result, e.err
}

func (e *onboardingAgentFakeEngine) Resume(_ context.Context, _ platform.Definition[workflow.OnboardingState], _ string, _ []byte) (platform.RunResult[workflow.OnboardingState], error) {
	return e.result, e.err
}

type onboardingAgentStartFailingEngine struct {
	startErr error
}

func (e *onboardingAgentStartFailingEngine) Start(_ context.Context, _ platform.Definition[workflow.OnboardingState], _ string, _ workflow.OnboardingState) (platform.RunResult[workflow.OnboardingState], error) {
	return platform.RunResult[workflow.OnboardingState]{}, e.startErr
}

func (e *onboardingAgentStartFailingEngine) Resume(_ context.Context, _ platform.Definition[workflow.OnboardingState], _ string, _ []byte) (platform.RunResult[workflow.OnboardingState], error) {
	return platform.RunResult[workflow.OnboardingState]{}, nil
}

type alwaysInProgressChecker struct{}

func (a *alwaysInProgressChecker) Check(_ context.Context, _ uuid.UUID) (bool, valueobjects.OnboardingPhase, error) {
	return true, valueobjects.PhaseWelcome, nil
}

type OnboardingAgentSuite struct {
	suite.Suite
	ctx   context.Context
	obs   *fake.Provider
	store *onboardingAgentFakeStore
}

func TestOnboardingAgentSuite(t *testing.T) {
	suite.Run(t, new(OnboardingAgentSuite))
}

func (s *OnboardingAgentSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.store = newOnboardingAgentFakeStore()
}

func (s *OnboardingAgentSuite) newAgent(engine platform.Engine[workflow.OnboardingState]) *OnboardingAgent {
	return NewOnboardingAgent(
		s.obs,
		s.obs.Metrics().Counter("agent_intent_routed_total", "", "1"),
		engine,
		platform.Definition[workflow.OnboardingState]{
			ID:   "onboarding",
			Root: platform.Sequence[workflow.OnboardingState]("onboarding.root"),
		},
		s.store,
		&alwaysInProgressChecker{},
		nil,
	)
}

func (s *OnboardingAgentSuite) TestHandle_Suspended_EmitsRoutedMetric() {
	engine := &onboardingAgentFakeEngine{
		result: platform.RunResult[workflow.OnboardingState]{
			RunID:  uuid.New(),
			Status: platform.RunStatusSuspended,
			State:  workflow.OnboardingState{Phase: valueobjects.PhaseObjective},
			Suspend: &platform.Suspension{
				Reason: platform.SuspendAwaitingInput,
				Prompt: "objective",
			},
		},
	}
	agent := s.newAgent(engine)

	res, ok := agent.Handle(s.ctx, uuid.New(), "whatsapp", "+5511999999999", "viajar", "m1")
	s.True(ok)
	s.Equal("objective", res.Reply)
	s.Equal(tools.OutcomeRouted, res.Outcome)
	s.Equal(intent.KindConfigureBudget, res.Kind)

	counter := s.obs.Metrics().(*fake.FakeMetrics).GetCounter("agent_intent_routed_total")
	s.Require().NotNil(counter)
	s.Len(counter.GetValues(), 1)
}

func (s *OnboardingAgentSuite) TestHandle_Succeeded_EmitsCompletedAndDurationMetrics() {
	engine := &onboardingAgentFakeEngine{
		result: platform.RunResult[workflow.OnboardingState]{
			RunID:  uuid.New(),
			Status: platform.RunStatusSucceeded,
			State:  workflow.OnboardingState{Phase: valueobjects.PhaseConclusion},
			Suspend: &platform.Suspension{
				Reason: platform.SuspendAwaitingInput,
				Prompt: "pronto",
			},
		},
	}
	agent := s.newAgent(engine)

	res, ok := agent.Handle(s.ctx, uuid.New(), "whatsapp", "+5511999999999", "sim", "m1")
	s.True(ok)
	s.Equal("pronto", res.Reply)

	completedCounter := s.obs.Metrics().(*fake.FakeMetrics).GetCounter("onboarding_completed_total")
	s.Require().NotNil(completedCounter)
	s.Len(completedCounter.GetValues(), 1)
	s.Equal(int64(1), completedCounter.GetValues()[0].Value)

	durationHist := s.obs.Metrics().(*fake.FakeMetrics).GetHistogram("onboarding_run_duration_seconds")
	s.Require().NotNil(durationHist)
	s.Len(durationHist.GetValues(), 1)
}

func (s *OnboardingAgentSuite) TestHandle_LogsRunAuditFields() {
	engine := &onboardingAgentFakeEngine{
		result: platform.RunResult[workflow.OnboardingState]{
			RunID:  uuid.New(),
			Status: platform.RunStatusSuspended,
			State:  workflow.OnboardingState{Phase: valueobjects.PhaseBudget},
			Suspend: &platform.Suspension{
				Reason: platform.SuspendAwaitingInput,
				Prompt: "budget",
			},
		},
	}
	agent := s.newAgent(engine)
	userID := uuid.New()

	_, _ = agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "4000", "m1")

	entries := s.obs.Logger().(*fake.FakeLogger).GetEntries()
	var found bool
	for _, e := range entries {
		if e.Message != "agent.onboarding.run" {
			continue
		}
		found = true
		s.True(hasRunAuditField(e.Fields, "thread_id", userID.String()))
		s.True(hasRunAuditField(e.Fields, "workflow", "onboarding"))
		s.True(hasRunAuditField(e.Fields, "step", "budget"))
		s.True(hasRunAuditField(e.Fields, "status", "suspended"))
		s.True(hasRunAuditField(e.Fields, "duration_ms", nil))
	}
	s.True(found)
}

func (s *OnboardingAgentSuite) TestHandle_Replay_DetectedByMessageID() {
	userID := uuid.New()
	key := userID.String()
	state := workflow.OnboardingState{UserID: userID, Phase: valueobjects.PhaseBudget, MessageID: "m1", ProcessedMessageIDs: []string{"m1"}}
	stateBytes, _ := json.Marshal(state)
	s.Require().NoError(s.store.Insert(s.ctx, platform.Snapshot{
		RunID:          uuid.New(),
		Workflow:       "onboarding",
		CorrelationKey: key,
		Status:         platform.RunStatusSuspended,
		State:          stateBytes,
		Version:        1,
	}))

	agent := s.newAgent(&onboardingAgentFakeEngine{})
	res, ok := agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "4000", "m1")
	s.True(ok)
	s.Equal(tools.OutcomeReplay, res.Outcome)
}

func (s *OnboardingAgentSuite) TestHandle_Replay_DetectedAfterNewerMessage() {
	userID := uuid.New()
	key := userID.String()
	state := workflow.OnboardingState{
		UserID:              userID,
		Phase:               valueobjects.PhaseBudget,
		MessageID:           "m2",
		ProcessedMessageIDs: []string{"m1", "m2"},
	}
	stateBytes, _ := json.Marshal(state)
	s.Require().NoError(s.store.Insert(s.ctx, platform.Snapshot{
		RunID:          uuid.New(),
		Workflow:       "onboarding",
		CorrelationKey: key,
		Status:         platform.RunStatusSuspended,
		State:          stateBytes,
		Version:        1,
	}))

	agent := s.newAgent(&onboardingAgentFakeEngine{})
	res, ok := agent.Handle(s.ctx, userID, "whatsapp", "+5511999999999", "4000", "m1")
	s.True(ok)
	s.Equal(tools.OutcomeReplay, res.Outcome)
}

func (s *OnboardingAgentSuite) TestHandle_StartFailure_ReportsRealError() {
	startErr := errors.New("engine start failed")
	engine := &onboardingAgentStartFailingEngine{startErr: startErr}
	agent := s.newAgent(engine)

	res, ok := agent.Handle(s.ctx, uuid.New(), "whatsapp", "+5511999999999", "sim", "m1")
	s.True(ok)
	s.Equal(tools.OutcomeUsecaseError, res.Outcome)

	entries := s.obs.Logger().(*fake.FakeLogger).GetEntries()
	var found bool
	for _, e := range entries {
		if e.Message != "agent.onboarding.start_failed" {
			continue
		}
		found = true
		s.True(hasRunAuditField(e.Fields, "error", startErr))
	}
	s.True(found)
}

func hasRunAuditField(fields []observability.Field, key string, expected any) bool {
	for _, f := range fields {
		if f.Key != key {
			continue
		}
		if expected == nil {
			return true
		}
		return f.AnyValue() == expected
	}
	return false
}
