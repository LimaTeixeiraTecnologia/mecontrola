//go:build integration

package binding_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type PendingEntryIntegrationSuite struct {
	suite.Suite
	ctx    context.Context
	engine workflow.Engine[workflows.PendingEntryState]
	def    workflow.Definition[workflows.PendingEntryState]
	store  workflow.Store
	ledger *integNoopLedger
	userID uuid.UUID
}

func TestPendingEntryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(PendingEntryIntegrationSuite))
}

func (s *PendingEntryIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	db, _ := testcontainer.Postgres(s.T())
	obs := fake.NewProvider()

	s.store = workflowpg.NewPostgresStore(obs, db)
	s.engine = workflow.NewEngine[workflows.PendingEntryState](s.store, obs)
	s.ledger = &integNoopLedger{}
	s.def = workflows.BuildPendingEntryWorkflow(s.ledger, nil, nil, nil)
	s.userID = uuid.New()
}

func (s *PendingEntryIntegrationSuite) newKey() string {
	return workflows.PendingEntryKey(s.userID.String()+"-"+uuid.New().String(), "thread-integ")
}

func (s *PendingEntryIntegrationSuite) singleCandidate() []workflows.PendingCategoryCandidate {
	return []workflows.PendingCategoryCandidate{{
		RootCategoryID:  uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62"),
		RootSlug:        "custo-fixo",
		SubcategoryID:   uuid.MustParse("97fa4b86-d43c-5ad5-a99b-c88c8427fb30"),
		SubcategorySlug: "supermercado",
		Path:            "Custo Fixo > Supermercado",
		Score:           1.0,
		Confidence:      "high",
	}}
}

func (s *PendingEntryIntegrationSuite) newState(awaiting workflows.AwaitingSlot, candidates []workflows.PendingCategoryCandidate) workflows.PendingEntryState {
	return workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      awaiting,
		OperationKind: workflows.PendingOpRegisterExpense,
		UserID:        s.userID,
		ResourceID:    s.userID,
		ThreadID:      "thread-integ",
		MessageID:     "wamid-integ-001",
		AmountCents:   15000,
		Description:   "supermercado",
		PaymentMethod: "pix",
		Kind:          agentsifaces.CategoryKindExpense,
		Candidates:    candidates,
		OccurredAt:    time.Now().UTC().Format("2006-01-02"),
		SuspendedAt:   time.Now().UTC(),
	}
}

func (s *PendingEntryIntegrationSuite) TestInteg_StartResume_Write_SnapshotSucceeded() {
	key := s.newKey()

	state := s.newState(workflows.AwaitingSlotConfirmation, s.singleCandidate())
	startResult, err := s.engine.Start(s.ctx, s.def, key, state)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, startResult.Status)

	patch, _ := json.Marshal(map[string]string{"resumeText": "sim"})
	result, err := s.engine.Resume(s.ctx, s.def, key, patch)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(workflows.PendingStatusCompleted, result.State.Status)

	_, found, err := s.store.Load(s.ctx, workflows.PendingEntryWorkflowID, key)
	s.Require().NoError(err)
	s.False(found, "run concluído deixa o conjunto ativo (Load só retorna running/suspended)")

	s.Equal(1, s.ledger.createCalls, "exactly 1 CreateTransaction call")
}

func (s *PendingEntryIntegrationSuite) TestInteg_ExpiredSnapshot_NoWrite() {
	key := s.newKey()

	codec := workflow.NewCodec[workflows.PendingEntryState]()
	expiredState := s.newState(workflows.AwaitingSlotConfirmation, s.singleCandidate())
	expiredState.SuspendedAt = time.Now().UTC().Add(-31 * time.Minute)
	encoded, err := codec.Encode(expiredState)
	require.NoError(s.T(), err)

	snap := workflow.Snapshot{
		RunID:          uuid.New(),
		Workflow:       workflows.PendingEntryWorkflowID,
		CorrelationKey: key,
		Status:         workflow.RunStatusSuspended,
		Version:        1,
		MaxAttempts:    1,
		State:          encoded,
	}
	require.NoError(s.T(), s.store.Insert(s.ctx, snap))

	patch, _ := json.Marshal(map[string]string{"resumeText": "sim"})
	result, err := s.engine.Resume(s.ctx, s.def, key, patch)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(workflows.PendingStatusExpired, result.State.Status)
	s.Contains(result.State.ResponseText, "expirou")
	s.Equal(0, s.ledger.createCalls, "no CreateTransaction on expiration")
}

func (s *PendingEntryIntegrationSuite) TestInteg_Substitution_NoWrite() {
	key := s.newKey()

	before := s.ledger.createCalls
	state := s.newState(workflows.AwaitingSlotCategory, nil)
	_, err := s.engine.Start(s.ctx, s.def, key, state)
	s.Require().NoError(err)

	patch, _ := json.Marshal(map[string]string{"resumeText": "Gastei R$ 50,00 na farmácia, pix"})
	result, err := s.engine.Resume(s.ctx, s.def, key, patch)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(workflows.PendingStatusReplaced, result.State.Status)
	s.Equal(before, s.ledger.createCalls, "no CreateTransaction on substitution")
}

func (s *PendingEntryIntegrationSuite) TestInteg_Cancel_NoWrite() {
	key := s.newKey()

	before := s.ledger.createCalls
	state := s.newState(workflows.AwaitingSlotConfirmation, s.singleCandidate())
	_, err := s.engine.Start(s.ctx, s.def, key, state)
	s.Require().NoError(err)

	patch, _ := json.Marshal(map[string]string{"resumeText": "não"})
	result, err := s.engine.Resume(s.ctx, s.def, key, patch)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(workflows.PendingStatusCancelled, result.State.Status)
	s.Equal(before, s.ledger.createCalls, "no CreateTransaction on cancel")
}

// ─── integNoopLedger ─────────────────────────────────────────────────────────

type integNoopLedger struct {
	createCalls int
	updateCalls int
	recurCalls  int
}

func (f *integNoopLedger) CreateTransaction(_ context.Context, _ agentsifaces.RawTransaction) (agentsifaces.EntryRef, error) {
	f.createCalls++
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindTransaction}, nil
}

func (f *integNoopLedger) UpdateTransaction(_ context.Context, _ agentsifaces.RawUpdateTransaction) (agentsifaces.EntryRef, error) {
	f.updateCalls++
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindTransaction}, nil
}

func (f *integNoopLedger) CreateRecurringTemplate(_ context.Context, _ agentsifaces.RawRecurringTemplate) (agentsifaces.EntryRef, error) {
	f.recurCalls++
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindRecurringTemplate}, nil
}

func (f *integNoopLedger) DeleteTransaction(_ context.Context, _ agentsifaces.EntryRef, _ int64) error {
	return nil
}

func (f *integNoopLedger) ListMonthlyEntries(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]agentsifaces.MonthlyEntry, error) {
	return nil, nil
}

func (f *integNoopLedger) GetMonthlySummary(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.MonthlySummary, error) {
	return agentsifaces.MonthlySummary{}, nil
}

func (f *integNoopLedger) GetCardInvoice(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.CardInvoice, error) {
	return agentsifaces.CardInvoice{}, nil
}

func (f *integNoopLedger) SearchTransactions(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]agentsifaces.Entry, error) {
	return nil, nil
}

func (f *integNoopLedger) GetTransaction(_ context.Context, _ string) (agentsifaces.Entry, error) {
	return agentsifaces.Entry{}, nil
}
