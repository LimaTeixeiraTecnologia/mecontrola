package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	agentpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/mocks"
	memorymocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type treatmentNameEditFakeStore struct {
	snapshots map[string]workflow.Snapshot
}

func newTreatmentNameEditFakeStore() *treatmentNameEditFakeStore {
	return &treatmentNameEditFakeStore{snapshots: map[string]workflow.Snapshot{}}
}

func (f *treatmentNameEditFakeStore) storeKey(workflowID, correlationKey string) string {
	return workflowID + "|" + correlationKey
}

func (f *treatmentNameEditFakeStore) Insert(_ context.Context, snap workflow.Snapshot) error {
	k := f.storeKey(snap.Workflow, snap.CorrelationKey)
	if existing, ok := f.snapshots[k]; ok && (existing.Status == workflow.RunStatusRunning || existing.Status == workflow.RunStatusSuspended) {
		return workflow.ErrRunAlreadyExists
	}
	f.snapshots[k] = snap
	return nil
}

func (f *treatmentNameEditFakeStore) Load(_ context.Context, workflowID, key string) (workflow.Snapshot, bool, error) {
	snap, ok := f.snapshots[f.storeKey(workflowID, key)]
	return snap, ok, nil
}

func (f *treatmentNameEditFakeStore) LoadLatest(ctx context.Context, workflowID, key string) (workflow.Snapshot, bool, error) {
	return f.Load(ctx, workflowID, key)
}

func (f *treatmentNameEditFakeStore) Save(_ context.Context, snap workflow.Snapshot, expectedVersion int64) error {
	k := f.storeKey(snap.Workflow, snap.CorrelationKey)
	if existing, ok := f.snapshots[k]; ok && existing.Version != expectedVersion {
		return workflow.ErrVersionConflict
	}
	f.snapshots[k] = snap
	return nil
}

func (f *treatmentNameEditFakeStore) AppendStep(_ context.Context, _ workflow.StepRecord) error {
	return nil
}

func (f *treatmentNameEditFakeStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

func (f *treatmentNameEditFakeStore) ListSuspended(_ context.Context, _ string, _ time.Time, _ int) ([]workflow.Snapshot, error) {
	return nil, nil
}

func (f *treatmentNameEditFakeStore) backdateSuspendedAt(workflowID, key string, at time.Time) {
	k := f.storeKey(workflowID, key)
	snap, ok := f.snapshots[k]
	if !ok {
		return
	}
	var state TreatmentNameEditState
	if err := json.Unmarshal(snap.State, &state); err != nil {
		return
	}
	state.SuspendedAt = at
	encoded, err := json.Marshal(state)
	if err != nil {
		return
	}
	snap.State = encoded
	f.snapshots[k] = snap
}

type TreatmentNameEditWorkflowSuite struct {
	suite.Suite
	ctx        context.Context
	wmMock     *memorymocks.WorkingMemory
	agentMock  *agentmocks.Agent
	resourceID string
}

func TestTreatmentNameEditWorkflowSuite(t *testing.T) {
	suite.Run(t, new(TreatmentNameEditWorkflowSuite))
}

func (s *TreatmentNameEditWorkflowSuite) SetupTest() {
	s.ctx = context.Background()
	s.wmMock = memorymocks.NewWorkingMemory(s.T())
	s.agentMock = agentmocks.NewAgent(s.T())
	s.resourceID = "user-1"
}

func (s *TreatmentNameEditWorkflowSuite) TestBuildTreatmentNameEditWorkflow_Definition() {
	def := BuildTreatmentNameEditWorkflow(s.wmMock, s.agentMock)
	s.Equal(TreatmentNameEditWorkflowID, def.ID)
	s.True(def.Durable)
	s.Equal(1, def.MaxAttempts)
	s.NotNil(def.Root)
}

func (s *TreatmentNameEditWorkflowSuite) TestBuildTreatmentNameEditKey() {
	s.Equal("res:thr:treatment-name-edit", TreatmentNameEditKey("res", "thr"))
}

func (s *TreatmentNameEditWorkflowSuite) TestProvidedNameAppliesInSingleTurn() {
	s.wmMock.EXPECT().
		Get(mock.Anything, s.resourceID).
		Return("## Objetivo Financeiro\n\nComprar casa", nil).Once()
	s.wmMock.EXPECT().
		Upsert(mock.Anything, s.resourceID, mock.MatchedBy(func(content string) bool {
			return strings.Contains(content, "## Nome de Tratamento") &&
				strings.Contains(content, "Stef") &&
				strings.Contains(content, "## Objetivo Financeiro")
		})).
		Return(nil).Once()
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, s.resourceID, map[string]any{"nome_tratamento": "Stef"}).
		Return(nil).Once()

	def := BuildTreatmentNameEditWorkflow(s.wmMock, s.agentMock)
	state := TreatmentNameEditState{ResourceID: s.resourceID, ProvidedName: "Stef"}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(TreatmentNameEditCompleted, out.State.Status)
	s.Equal("Combinado, Stef! 💚 Vou te chamar assim daqui pra frente.", out.State.ResponseText)
}

func (s *TreatmentNameEditWorkflowSuite) TestNoNameSuspendsAskingQuestion() {
	def := BuildTreatmentNameEditWorkflow(s.wmMock, s.agentMock)
	state := TreatmentNameEditState{ResourceID: s.resourceID}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.NotNil(out.Suspend)
	s.Equal("Claro! Como você gostaria que eu te chamasse a partir de agora? 💚", out.Suspend.Prompt)
	s.False(out.State.SuspendedAt.IsZero())
}

func (s *TreatmentNameEditWorkflowSuite) TestResumeWithUsableNameExtractsAndApplies() {
	payload, marshalErr := json.Marshal(treatmentNameEditExtract{HasName: true, Name: "Stef"})
	s.Require().NoError(marshalErr)
	s.agentMock.EXPECT().
		Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(agentpkg.Result{RawJSON: payload}, nil).Once()
	s.wmMock.EXPECT().Get(mock.Anything, s.resourceID).Return("", nil).Once()
	s.wmMock.EXPECT().
		Upsert(mock.Anything, s.resourceID, "## Nome de Tratamento\n\nStef").
		Return(nil).Once()
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, s.resourceID, map[string]any{"nome_tratamento": "Stef"}).
		Return(nil).Once()

	def := BuildTreatmentNameEditWorkflow(s.wmMock, s.agentMock)
	state := TreatmentNameEditState{
		ResourceID:  s.resourceID,
		SuspendedAt: time.Now().UTC(),
		ResumeText:  "pode me chamar de Stef",
	}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(TreatmentNameEditCompleted, out.State.Status)
	s.Equal("Combinado, Stef! 💚 Vou te chamar assim daqui pra frente.", out.State.ResponseText)
}

func (s *TreatmentNameEditWorkflowSuite) TestResumeUnusableRepromptsOnceThenCancels() {
	first, marshalErr := json.Marshal(treatmentNameEditExtract{HasName: false})
	s.Require().NoError(marshalErr)
	s.agentMock.EXPECT().
		Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(agentpkg.Result{RawJSON: first}, nil).Once()

	def := BuildTreatmentNameEditWorkflow(s.wmMock, s.agentMock)
	state := TreatmentNameEditState{
		ResourceID:  s.resourceID,
		SuspendedAt: time.Now().UTC(),
		ResumeText:  "não sei",
	}

	out, err := def.Root.Execute(s.ctx, state)
	s.NoError(err)
	s.Equal(workflow.StepStatusSuspended, out.Status)
	s.Equal(1, out.State.RepromptCount)
	s.Equal(treatmentNameEditReprompt(), out.Suspend.Prompt)

	second, marshalErr := json.Marshal(treatmentNameEditExtract{HasName: false})
	s.Require().NoError(marshalErr)
	s.agentMock.EXPECT().
		Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(agentpkg.Result{RawJSON: second}, nil).Once()

	state2 := out.State
	state2.ResumeText = "sei lá"

	out2, err2 := def.Root.Execute(s.ctx, state2)
	s.NoError(err2)
	s.Equal(workflow.StepStatusCompleted, out2.Status)
	s.Equal(TreatmentNameEditCancelled, out2.State.Status)
	s.Contains(out2.State.ResponseText, "Tudo bem")
}

func (s *TreatmentNameEditWorkflowSuite) TestExpiredResumeCompletesWithoutApplying() {
	def := BuildTreatmentNameEditWorkflow(s.wmMock, s.agentMock)
	old := time.Now().UTC().Add(-30 * time.Minute)
	state := TreatmentNameEditState{ResourceID: s.resourceID, SuspendedAt: old, ResumeText: "Stef"}

	out, err := def.Root.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(workflow.StepStatusCompleted, out.Status)
	s.Equal(TreatmentNameEditExpired, out.State.Status)
	s.True(out.State.Expired)
	s.Empty(out.State.ResponseText)
	s.Empty(out.State.ResumeText)
}

func (s *TreatmentNameEditWorkflowSuite) TestUpsertFailureReturnsFailedStatusWithoutConfirming() {
	s.wmMock.EXPECT().Get(mock.Anything, s.resourceID).Return("", nil).Once()
	s.wmMock.EXPECT().
		Upsert(mock.Anything, s.resourceID, mock.AnythingOfType("string")).
		Return(errors.New("db down")).Once()

	def := BuildTreatmentNameEditWorkflow(s.wmMock, s.agentMock)
	state := TreatmentNameEditState{ResourceID: s.resourceID, ProvidedName: "Stef"}

	out, err := def.Root.Execute(s.ctx, state)

	s.Error(err)
	s.Equal(workflow.StepStatusFailed, out.Status)
	s.NotEqual("Combinado, Stef! 💚 Vou te chamar assim daqui pra frente.", out.State.ResponseText)
	s.NotEmpty(out.State.ResponseText)
}

func (s *TreatmentNameEditWorkflowSuite) TestUpsertMetadataFailureReturnsFailedStatus() {
	s.wmMock.EXPECT().Get(mock.Anything, s.resourceID).Return("", nil).Once()
	s.wmMock.EXPECT().
		Upsert(mock.Anything, s.resourceID, mock.AnythingOfType("string")).
		Return(nil).Once()
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, s.resourceID, map[string]any{"nome_tratamento": "Stef"}).
		Return(errors.New("db down")).Once()

	def := BuildTreatmentNameEditWorkflow(s.wmMock, s.agentMock)
	state := TreatmentNameEditState{ResourceID: s.resourceID, ProvidedName: "Stef"}

	out, err := def.Root.Execute(s.ctx, state)

	s.Error(err)
	s.Equal(workflow.StepStatusFailed, out.Status)
}

func (s *TreatmentNameEditWorkflowSuite) TestBuildTreatmentNameEditReaper() {
	reaper := BuildTreatmentNameEditReaper(nil, fake.NewProvider())
	s.NotNil(reaper)
}

type TreatmentNameEditContinueSuite struct {
	suite.Suite
	ctx        context.Context
	store      *treatmentNameEditFakeStore
	wmMock     *memorymocks.WorkingMemory
	agentMock  *agentmocks.Agent
	resourceID string
	threadID   string
	key        string
	engine     workflow.Engine[TreatmentNameEditState]
	def        workflow.Definition[TreatmentNameEditState]
}

func TestTreatmentNameEditContinueSuite(t *testing.T) {
	suite.Run(t, new(TreatmentNameEditContinueSuite))
}

func (s *TreatmentNameEditContinueSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = newTreatmentNameEditFakeStore()
	s.wmMock = memorymocks.NewWorkingMemory(s.T())
	s.agentMock = agentmocks.NewAgent(s.T())
	s.resourceID = "user-1"
	s.threadID = "thread-1"
	s.key = TreatmentNameEditKey(s.resourceID, s.threadID)
	s.def = BuildTreatmentNameEditWorkflow(s.wmMock, s.agentMock)
	s.engine = workflow.NewEngine[TreatmentNameEditState](s.store, fake.NewProvider())
}

func (s *TreatmentNameEditContinueSuite) TestNoSuspendedRunIsNotHandled() {
	handled, responseText, err := ContinueTreatmentNameEdit(s.ctx, s.engine, s.def, s.key, "qualquer coisa")

	s.NoError(err)
	s.False(handled)
	s.Empty(responseText)
}

func (s *TreatmentNameEditContinueSuite) TestExpiredRunReturnsNotHandled() {
	_, startErr := s.engine.Start(s.ctx, s.def, s.key, TreatmentNameEditState{ResourceID: s.resourceID})
	s.Require().NoError(startErr)
	s.store.backdateSuspendedAt(TreatmentNameEditWorkflowID, s.key, time.Now().UTC().Add(-30*time.Minute))

	handled, responseText, err := ContinueTreatmentNameEdit(s.ctx, s.engine, s.def, s.key, "Stef")

	s.NoError(err)
	s.False(handled)
	s.Empty(responseText)
}

func (s *TreatmentNameEditContinueSuite) TestResumeAppliesNameAndReturnsConfirmation() {
	_, startErr := s.engine.Start(s.ctx, s.def, s.key, TreatmentNameEditState{ResourceID: s.resourceID})
	s.Require().NoError(startErr)

	payload, marshalErr := json.Marshal(treatmentNameEditExtract{HasName: true, Name: "Stef"})
	s.Require().NoError(marshalErr)
	s.agentMock.EXPECT().
		Execute(mock.Anything, mock.AnythingOfType("agent.Request")).
		Return(agentpkg.Result{RawJSON: payload}, nil).Once()
	s.wmMock.EXPECT().Get(mock.Anything, s.resourceID).Return("", nil).Once()
	s.wmMock.EXPECT().
		Upsert(mock.Anything, s.resourceID, "## Nome de Tratamento\n\nStef").
		Return(nil).Once()
	s.wmMock.EXPECT().
		UpsertMetadata(mock.Anything, s.resourceID, map[string]any{"nome_tratamento": "Stef"}).
		Return(nil).Once()

	handled, responseText, err := ContinueTreatmentNameEdit(s.ctx, s.engine, s.def, s.key, "pode me chamar de Stef")

	s.NoError(err)
	s.True(handled)
	s.Equal("Combinado, Stef! 💚 Vou te chamar assim daqui pra frente.", responseText)
}
