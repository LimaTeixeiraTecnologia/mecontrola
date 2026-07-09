//go:build integration

package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	agentpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/infrastructure/postgres"
	mempostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type continuerFailingLedger struct {
	forceErr error
}

func (f *continuerFailingLedger) CreateTransaction(_ context.Context, _ agentsifaces.RawTransaction) (agentsifaces.EntryRef, error) {
	if f.forceErr != nil {
		return agentsifaces.EntryRef{}, f.forceErr
	}
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindTransaction}, nil
}

func (f *continuerFailingLedger) UpdateTransaction(_ context.Context, _ agentsifaces.RawUpdateTransaction) (agentsifaces.EntryRef, error) {
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindTransaction}, nil
}

func (f *continuerFailingLedger) CreateRecurringTemplate(_ context.Context, _ agentsifaces.RawRecurringTemplate) (agentsifaces.EntryRef, error) {
	return agentsifaces.EntryRef{ID: uuid.New(), Kind: agentsifaces.EntryKindRecurringTemplate}, nil
}

func (f *continuerFailingLedger) DeleteTransaction(_ context.Context, _ agentsifaces.EntryRef, _ int64) error {
	return nil
}

func (f *continuerFailingLedger) ListMonthlyEntries(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]agentsifaces.MonthlyEntry, error) {
	return nil, nil
}

func (f *continuerFailingLedger) GetMonthlySummary(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.MonthlySummary, error) {
	return agentsifaces.MonthlySummary{}, nil
}

func (f *continuerFailingLedger) GetCardInvoice(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.CardInvoice, error) {
	return agentsifaces.CardInvoice{}, nil
}

func (f *continuerFailingLedger) SearchTransactions(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]agentsifaces.Entry, error) {
	return nil, nil
}

func (f *continuerFailingLedger) GetTransaction(_ context.Context, _ string) (agentsifaces.Entry, error) {
	return agentsifaces.Entry{}, nil
}

type PendingEntryContinuerIntegrationSuite struct {
	suite.Suite
	ctx context.Context
}

func TestPendingEntryContinuerIntegrationSuite(t *testing.T) {
	suite.Run(t, new(PendingEntryContinuerIntegrationSuite))
}

func (s *PendingEntryContinuerIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
}

func (s *PendingEntryContinuerIntegrationSuite) TestInteg_EscritaFalhaNoResume_GravaErroRealEmAmbasTabelas() {
	db, _ := testcontainer.Postgres(s.T())
	obs := fake.NewProvider()

	threads := mempostgres.NewThreadRepository(db, obs)
	runs := agentpostgres.NewRunStore(db)
	workflowStore := workflowpg.NewPostgresStore(obs, db)
	engine := workflow.NewEngine[workflows.PendingEntryState](workflowStore, obs)

	ledger := &continuerFailingLedger{forceErr: errors.New("db unavailable")}
	def := workflows.BuildPendingEntryWorkflow(ledger, nil, nil, nil)

	userID := uuid.New()
	threadID := "thread-integ-" + uuid.NewString()
	wamid := "wamid-integ-" + uuid.NewString()

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
		Candidates: []workflows.PendingCategoryCandidate{{
			RootCategoryID:  uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62"),
			RootSlug:        "custo-fixo",
			SubcategoryID:   uuid.MustParse("97fa4b86-d43c-5ad5-a99b-c88c8427fb30"),
			SubcategorySlug: "supermercado",
			Path:            "Custo Fixo > Supermercado",
		}},
		OccurredAt:  time.Now().UTC().Format("2006-01-02"),
		SuspendedAt: time.Now().UTC(),
	}

	startResult, err := engine.Start(s.ctx, def, key, state)
	s.Require().NoError(err)
	s.Require().Equal(workflow.RunStatusSuspended, startResult.Status)

	continuer := usecases.NewPendingEntryContinuer(engine, def, threads, runs, obs)
	_, contErr := continuer.Continue(s.ctx, userID.String(), threadID, "sim", wamid+"-resume")

	s.Require().Error(contErr, "RF-10: escrita falha no resume deve propagar erro real, nunca ser engolida")

	var runError, correlationKey, runStatus string
	scanErr := db.QueryRowContext(s.ctx,
		`SELECT error, correlation_key, status FROM mecontrola.platform_runs WHERE resource_id = $1 AND thread_id = $2 ORDER BY started_at DESC LIMIT 1`,
		userID.String(), threadID,
	).Scan(&runError, &correlationKey, &runStatus)
	s.Require().NoError(scanErr)
	s.NotEmpty(runError, "RF-10: platform_runs.error deve estar preenchido na falha real")
	s.Contains(runError, "db unavailable", "RF-11: erro real deve estar no platform_runs.error")
	s.Equal("failed", runStatus)
	s.Equal(wamid+"-resume", correlationKey, "RF-12: wamid deve estar correlacionado no run auditável")

	var lastError, wfStatus string
	scanErr = db.QueryRowContext(s.ctx,
		`SELECT last_error, status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.PendingEntryWorkflowID, key,
	).Scan(&lastError, &wfStatus)
	s.Require().NoError(scanErr)
	s.NotEmpty(lastError, "RF-10: workflow_runs.last_error (mecanismo do kernel) deve estar preenchido")
	s.Equal("failed", wfStatus)
}

func (s *PendingEntryContinuerIntegrationSuite) TestInteg_Cancelamento_RunNaoFicaFailed() {
	db, _ := testcontainer.Postgres(s.T())
	obs := fake.NewProvider()

	threads := mempostgres.NewThreadRepository(db, obs)
	runs := agentpostgres.NewRunStore(db)
	workflowStore := workflowpg.NewPostgresStore(obs, db)
	engine := workflow.NewEngine[workflows.PendingEntryState](workflowStore, obs)

	ledger := &continuerFailingLedger{}
	def := workflows.BuildPendingEntryWorkflow(ledger, nil, nil, nil)

	userID := uuid.New()
	threadID := "thread-integ-" + uuid.NewString()
	wamid := "wamid-integ-" + uuid.NewString()

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
		Candidates: []workflows.PendingCategoryCandidate{{
			RootCategoryID:  uuid.MustParse("66cb85a0-3266-5900-b8e3-13cdcd00ab62"),
			RootSlug:        "custo-fixo",
			SubcategoryID:   uuid.MustParse("97fa4b86-d43c-5ad5-a99b-c88c8427fb30"),
			SubcategorySlug: "supermercado",
			Path:            "Custo Fixo > Supermercado",
		}},
		OccurredAt:  time.Now().UTC().Format("2006-01-02"),
		SuspendedAt: time.Now().UTC(),
	}

	startResult, err := engine.Start(s.ctx, def, key, state)
	s.Require().NoError(err)
	s.Require().Equal(workflow.RunStatusSuspended, startResult.Status)

	continuer := usecases.NewPendingEntryContinuer(engine, def, threads, runs, obs)
	result, contErr := continuer.Continue(s.ctx, userID.String(), threadID, "não", wamid+"-cancel")

	s.Require().NoError(contErr)
	s.Equal(workflows.PendingEntryModeCancelled, result.Mode)

	var runStatus string
	scanErr := db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.platform_runs WHERE resource_id = $1 AND thread_id = $2 ORDER BY started_at DESC LIMIT 1`,
		userID.String(), threadID,
	).Scan(&runStatus)
	s.Require().NoError(scanErr)
	s.NotEqual("failed", runStatus, "cancelamento explícito não é falha: run não deve ficar failed")
}
