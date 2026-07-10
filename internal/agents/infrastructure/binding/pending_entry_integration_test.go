//go:build integration

package binding_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	agentusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	agentpersistence "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type PendingEntryIntegrationSuite struct {
	suite.Suite
	ctx    context.Context
	db     *sqlx.DB
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
	s.db = db
	obs := fake.NewProvider()

	s.store = workflowpg.NewPostgresStore(obs, db)
	s.engine = workflow.NewEngine[workflows.PendingEntryState](s.store, obs)
	s.ledger = &integNoopLedger{}
	s.def = workflows.BuildPendingEntryWorkflow(s.ledger, nil, nil, nil)
	s.userID = uuid.New()
}

func (s *PendingEntryIntegrationSuite) newUser() uuid.UUID {
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, "+55119"+uuid.NewString()[:8],
	)
	s.Require().NoError(err)
	return userID
}

func (s *PendingEntryIntegrationSuite) buildEngineWithRealIdempotency() (
	workflow.Engine[workflows.PendingEntryState],
	workflow.Definition[workflows.PendingEntryState],
	*countingLedger,
) {
	o11y := fake.NewProvider()
	store := workflowpg.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.PendingEntryState](store, o11y)

	writeLedgerRepo := agentpersistence.NewWriteLedgerRepository(s.db, o11y)
	idem := agentusecases.NewIdempotentWrite(writeLedgerRepo, o11y)
	ledger := &countingLedger{}
	idemAdapter := pendingEntryIdemAdapter{uc: idem}

	def := workflows.BuildPendingEntryWorkflow(ledger, nil, nil, idemAdapter)
	return engine, def, ledger
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

func (s *PendingEntryIntegrationSuite) TestInteg_DuplicateWamid_RealIdempotentWriter_SingleEffect() {
	userID := s.newUser()
	engine, def, ledger := s.buildEngineWithRealIdempotency()

	threadID := "thread-idem-" + uuid.NewString()
	wamid := "wamid-idem-" + uuid.NewString()
	key := workflows.PendingEntryKey(userID.String(), threadID)

	state := workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      workflows.AwaitingSlotConfirmation,
		OperationKind: workflows.PendingOpRegisterExpense,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      threadID,
		MessageID:     wamid,
		AmountCents:   15000,
		Description:   "supermercado",
		PaymentMethod: "pix",
		Kind:          agentsifaces.CategoryKindExpense,
		Candidates:    s.singleCandidate(),
		OccurredAt:    time.Now().UTC().Format("2006-01-02"),
		SuspendedAt:   time.Now().UTC(),
	}

	_, err := engine.Start(s.ctx, def, key, state)
	s.Require().NoError(err)

	patch, _ := json.Marshal(map[string]string{"resumeText": "sim"})
	result1, err1 := engine.Resume(s.ctx, def, key, patch)
	s.Require().NoError(err1)
	s.Equal(workflow.RunStatusSucceeded, result1.Status)
	s.Equal(workflows.PendingStatusCompleted, result1.State.Status)
	s.Equal(int32(1), ledger.createCalls.Load(), "primeira confirmação deve efetivar exatamente 1 escrita")

	stateReplay := state
	stateReplay.MessageID = wamid
	_, err = engine.Start(s.ctx, def, key, stateReplay)
	s.Require().NoError(err, "novo Start reabre a pendência (run anterior já concluído)")
	result2, err2 := engine.Resume(s.ctx, def, key, patch)
	s.Require().NoError(err2)
	s.Equal(workflow.RunStatusSucceeded, result2.Status)
	s.Equal(workflows.PendingStatusCompleted, result2.State.Status)

	s.Equal(int32(1), ledger.createCalls.Load(), "RF-14: replay do mesmo wamid não deve criar um segundo lançamento (WriteLedger absorve o conflito)")
	s.Contains(result2.State.ResponseText, "registrada", "RF-45: texto determinístico de repetição idempotente")
}

func (s *PendingEntryIntegrationSuite) TestInteg_ConcorrenciaResolucaoSimultanea_UmUnicoEfeito() {
	userID := s.newUser()
	engine, def, ledger := s.buildEngineWithRealIdempotency()

	threadID := "thread-concurrent-" + uuid.NewString()
	wamid := "wamid-concurrent-" + uuid.NewString()
	key := workflows.PendingEntryKey(userID.String(), threadID)

	state := workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      workflows.AwaitingSlotConfirmation,
		OperationKind: workflows.PendingOpRegisterExpense,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      threadID,
		MessageID:     wamid,
		AmountCents:   20000,
		Description:   "concorrencia",
		PaymentMethod: "pix",
		Kind:          agentsifaces.CategoryKindExpense,
		Candidates:    s.singleCandidate(),
		OccurredAt:    time.Now().UTC().Format("2006-01-02"),
		SuspendedAt:   time.Now().UTC(),
	}

	_, err := engine.Start(s.ctx, def, key, state)
	s.Require().NoError(err)

	patch, _ := json.Marshal(map[string]string{"resumeText": "sim"})

	const goroutines = 5
	var wg sync.WaitGroup
	wg.Add(goroutines)
	results := make([]workflow.RunResult[workflows.PendingEntryState], goroutines)
	errs := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = engine.Resume(s.ctx, def, key, patch)
		}(i)
	}
	wg.Wait()

	succeeded := 0
	for i := 0; i < goroutines; i++ {
		if errs[i] != nil {
			s.Contains(errs[i].Error(), "version conflict", "RF-45: única classe de erro aceitável em concorrência é o CAS otimista do kernel — perdedor nunca reexecuta")
			continue
		}
		if results[i].Status == workflow.RunStatusSucceeded {
			succeeded++
		}
	}
	s.GreaterOrEqual(succeeded, 1, "ao menos uma goroutine deve concluir o run")
	s.Equal(int32(1), ledger.createCalls.Load(), "concorrência sobre a mesma pendência deve produzir exatamente 1 efeito financeiro")
}

func (s *PendingEntryIntegrationSuite) TestInteg_Reaper_RunOrfaoEncerraFailedNuncaSuspended() {
	userID := s.newUser()
	engine, def, ledger := s.buildEngineWithRealIdempotency()

	threadID := "thread-reaper-" + uuid.NewString()
	wamid := "wamid-reaper-" + uuid.NewString()
	key := workflows.PendingEntryKey(userID.String(), threadID)

	state := workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      workflows.AwaitingSlotConfirmation,
		OperationKind: workflows.PendingOpRegisterExpense,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      threadID,
		MessageID:     wamid,
		AmountCents:   10000,
		Description:   "orfao",
		PaymentMethod: "pix",
		Kind:          agentsifaces.CategoryKindExpense,
		Candidates:    s.singleCandidate(),
		OccurredAt:    time.Now().UTC().Format("2006-01-02"),
		SuspendedAt:   time.Now().UTC(),
	}

	_, err := engine.Start(s.ctx, def, key, state)
	s.Require().NoError(err)

	_, execErr := s.db.ExecContext(s.ctx,
		`UPDATE mecontrola.workflow_runs SET updated_at = now() - interval '40 minutes' WHERE workflow = $1 AND correlation_key = $2`,
		workflows.PendingEntryWorkflowID, key,
	)
	s.Require().NoError(execErr)

	o11y := fake.NewProvider()
	store := workflowpg.NewPostgresStore(o11y, s.db)
	reaper := workflows.BuildPendingEntryReaper(store, o11y)

	reaped, reapErr := reaper.Reap(s.ctx)
	s.Require().NoError(reapErr)
	s.GreaterOrEqual(reaped, int64(1))

	var status string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.PendingEntryWorkflowID, key,
	).Scan(&status)
	s.Require().NoError(scanErr)
	s.Equal("failed", status, "RF-46: reaper deve marcar run órfão como failed, nunca permanecer suspended")

	s.Equal(int32(0), ledger.createCalls.Load(), "run órfão reapeado não deve produzir efeito financeiro")
}

// ─── pendingEntryIdemAdapter / countingLedger ────────────────────────────────

type pendingEntryIdemAdapter struct {
	uc *agentusecases.IdempotentWrite
}

func (a pendingEntryIdemAdapter) Execute(
	ctx context.Context,
	userID uuid.UUID,
	wamid string,
	itemSeq int,
	operation string,
	resourceKind string,
	write workflows.IdempotentWriteFn,
	isDomainErr workflows.DomainErrorClassifier,
) (uuid.UUID, agent.ToolOutcome, error) {
	res, err := a.uc.Execute(ctx, userID, wamid, itemSeq, operation, resourceKind, agentusecases.WriteFn(write), isDomainErr)
	return res.ResourceID, res.Outcome, err
}

type countingLedger struct {
	createCalls atomic.Int32
}

func (f *countingLedger) CreateTransaction(_ context.Context, _ agentsifaces.RawTransaction) (agentsifaces.EntryRef, error) {
	f.createCalls.Add(1)
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindTransaction}, nil
}

func (f *countingLedger) UpdateTransaction(_ context.Context, _ agentsifaces.RawUpdateTransaction) (agentsifaces.EntryRef, error) {
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindTransaction}, nil
}

func (f *countingLedger) CreateRecurringTemplate(_ context.Context, _ agentsifaces.RawRecurringTemplate) (agentsifaces.EntryRef, error) {
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindRecurringTemplate}, nil
}

func (f *countingLedger) DeleteTransaction(_ context.Context, _ agentsifaces.EntryRef, _ int64) error {
	return nil
}

func (f *countingLedger) ListMonthlyEntries(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]agentsifaces.MonthlyEntry, error) {
	return nil, nil
}

func (f *countingLedger) GetMonthlySummary(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.MonthlySummary, error) {
	return agentsifaces.MonthlySummary{}, nil
}

func (f *countingLedger) GetCardInvoice(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.CardInvoice, error) {
	return agentsifaces.CardInvoice{}, nil
}

func (f *countingLedger) SearchTransactions(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]agentsifaces.Entry, error) {
	return nil, nil
}

func (f *countingLedger) GetTransaction(_ context.Context, _ string) (agentsifaces.Entry, error) {
	return agentsifaces.Entry{}, nil
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
