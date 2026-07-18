//go:build integration

package binding_test

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	usecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type testIdempotentWriter struct {
	uc *usecases.IdempotentWrite
}

func (a testIdempotentWriter) Execute(
	ctx context.Context,
	userID uuid.UUID,
	wamid string,
	itemSeq int,
	operation string,
	resourceKind string,
	write workflows.IdempotentWriteFn,
	isDomainErr workflows.DomainErrorClassifier,
) (uuid.UUID, agent.ToolOutcome, error) {
	res, err := a.uc.Execute(ctx, userID, wamid, itemSeq, operation, resourceKind, usecases.WriteFn(write), isDomainErr)
	return res.ResourceID, res.Outcome, err
}

type stubResumeThreadGateway struct{}

func (stubResumeThreadGateway) GetOrCreate(_ context.Context, _, _ string) (memory.Thread, error) {
	return memory.Thread{ID: uuid.New()}, nil
}

type stubResumeRunStore struct{}

func (stubResumeRunStore) Insert(_ context.Context, _ agent.Run) error { return nil }
func (stubResumeRunStore) Update(_ context.Context, _ agent.Run) error { return nil }
func (stubResumeRunStore) Load(_ context.Context, _ uuid.UUID) (agent.Run, error) {
	return agent.Run{}, nil
}

func (s *TransactionsIntegrationSuite) buildTransactionWriteDispatcher() *usecases.ResumeDispatcher {
	o11y := fake.NewProvider()
	store := workflowpostgres.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.TransactionWriteState](store, o11y)
	def := workflows.BuildTransactionWriteWorkflowWithObservability(s.adapter, nil, nil, testIdempotentWriter{uc: s.idemUC}, o11y)

	registry := agent.NewWorkflowRegistry[workflows.TransactionWriteState]()
	registry.Register(def)

	resumer, err := usecases.NewWorkflowResumer(
		workflows.TransactionWriteWorkflowID, registry, engine,
		workflows.TransactionWriteKey, workflows.ContinueTransactionWrite,
	)
	s.Require().NoError(err)

	index := usecases.NewSuspendedRunIndex(store, workflows.TransactionWriteWorkflowID)
	dispatcher, err := usecases.NewResumeDispatcher(index, stubResumeThreadGateway{}, stubResumeRunStore{}, o11y, resumer)
	s.Require().NoError(err)
	return dispatcher
}

func (s *TransactionsIntegrationSuite) seedSuspendedIncomeConfirmation(userID uuid.UUID, threadID, wamid string) {
	o11y := fake.NewProvider()
	store := workflowpostgres.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.TransactionWriteState](store, o11y)
	def := workflows.BuildTransactionWriteWorkflowWithObservability(s.adapter, nil, nil, testIdempotentWriter{uc: s.idemUC}, o11y)

	var editorialVersion int64
	s.Require().NoError(
		s.db.QueryRowContext(s.ctx, `SELECT version FROM mecontrola.category_editorial_version LIMIT 1`).Scan(&editorialVersion),
	)

	state := workflows.TransactionWriteState{
		Status:        workflows.TransactionWriteStatusActive,
		Awaiting:      workflows.TransactionAwaitingConfirmation,
		OperationKind: workflows.TransactionOpRegisterIncome,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      threadID,
		MessageID:     wamid,
		AmountCents:   5000,
		Description:   "salário",
		PaymentMethod: "pix",
		Installments:  1,
		OccurredAt:    "2026-07-01",
		Kind:          agentsifaces.CategoryKindIncome,
		Candidates: []workflows.PendingCategoryCandidate{{
			RootCategoryID:  s.rootCatID,
			SubcategoryID:   s.leafCatID,
			Path:            "Salário > Bônus",
			RootSlug:        "integ-salario",
			SubcategorySlug: "integ-bonus",
		}},
		CategoryVersion: editorialVersion,
		SuspendedAt:     time.Now().UTC(),
	}

	key := workflows.TransactionWriteKey(userID.String(), threadID)
	result, err := engine.Start(s.ctx, def, key, state)
	s.Require().NoError(err)
	s.Require().Equal(workflow.RunStatusSuspended, result.Status)
}

func (s *TransactionsIntegrationSuite) countTransactions(userID uuid.UUID) int {
	var n int
	s.Require().NoError(
		s.db.QueryRowContext(s.ctx,
			`SELECT count(*) FROM mecontrola.transactions WHERE user_id=$1 AND deleted_at IS NULL`, userID,
		).Scan(&n),
	)
	return n
}

func (s *TransactionsIntegrationSuite) countWriteLedger(userID uuid.UUID) int {
	var n int
	s.Require().NoError(
		s.db.QueryRowContext(s.ctx,
			`SELECT count(*) FROM mecontrola.agents_write_ledger WHERE user_id=$1`, userID,
		).Scan(&n),
	)
	return n
}

func (s *TransactionsIntegrationSuite) TestResumeDispatcher_PersistsTransactionWithInboundIdentity() {
	userID := uuid.New()
	threadID := "+5511930000001"
	wamid := "wamid-resume-persist"
	s.seedSuspendedIncomeConfirmation(userID, threadID, wamid)

	s.Require().Equal(0, s.countTransactions(userID))

	dispatcher := s.buildTransactionWriteDispatcher()

	handled, reply, err := dispatcher.Continue(context.Background(), userID.String(), threadID, "sim", "wamid-inbound-sim")
	s.Require().NoError(err)
	s.True(handled)
	s.NotEmpty(reply)

	s.Equal(1, s.countTransactions(userID))
	s.Equal(1, s.countWriteLedger(userID))
}

func (s *TransactionsIntegrationSuite) TestResumeWithoutInboundIdentity_ReproducesProductionFailure() {
	userID := uuid.New()
	threadID := "+5511930000002"
	wamid := "wamid-resume-noidentity"
	s.seedSuspendedIncomeConfirmation(userID, threadID, wamid)

	o11y := fake.NewProvider()
	store := workflowpostgres.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.TransactionWriteState](store, o11y)
	def := workflows.BuildTransactionWriteWorkflowWithObservability(s.adapter, nil, nil, testIdempotentWriter{uc: s.idemUC}, o11y)

	handled, reply, err := workflows.ContinueTransactionWrite(
		context.Background(), engine, def,
		workflows.TransactionWriteKey(userID.String(), threadID), "sim", "wamid-inbound-noidentity",
	)

	s.Require().Error(err)
	s.Contains(err.Error(), "identidade inbound ausente")
	s.True(handled)
	s.NotEmpty(reply)
	s.Equal(0, s.countTransactions(userID))
	s.Equal(0, s.countWriteLedger(userID))
}

func (s *TransactionsIntegrationSuite) seedSuspendedExpenseAwaitingPayment(userID uuid.UUID, threadID, wamid string) {
	o11y := fake.NewProvider()
	store := workflowpostgres.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.TransactionWriteState](store, o11y)
	def := workflows.BuildTransactionWriteWorkflowWithObservability(s.adapter, nil, nil, testIdempotentWriter{uc: s.idemUC}, o11y)

	var editorialVersion int64
	s.Require().NoError(
		s.db.QueryRowContext(s.ctx, `SELECT version FROM mecontrola.category_editorial_version LIMIT 1`).Scan(&editorialVersion),
	)

	state := workflows.TransactionWriteState{
		Status:        workflows.TransactionWriteStatusActive,
		OperationKind: workflows.TransactionOpRegisterExpense,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      threadID,
		MessageID:     wamid,
		AmountCents:   3000,
		Description:   "padaria",
		Installments:  1,
		OccurredAt:    "2026-07-01",
		Kind:          agentsifaces.CategoryKindExpense,
		Candidates: []workflows.PendingCategoryCandidate{{
			RootCategoryID:  s.expenseRootID,
			SubcategoryID:   s.expenseLeafID,
			Path:            "Alimentação > Restaurante",
			RootSlug:        "integ-alimentacao",
			SubcategorySlug: "integ-restaurante",
		}},
		CategoryVersion: editorialVersion,
	}

	key := workflows.TransactionWriteKey(userID.String(), threadID)
	result, err := engine.Start(s.ctx, def, key, state)
	s.Require().NoError(err)
	s.Require().Equal(workflow.RunStatusSuspended, result.Status)
	s.Require().Equal(workflows.TransactionAwaitingPaymentMethod, result.State.Awaiting)
	s.Require().Equal(messages.ClarificationQuestion(messages.MissingFieldPaymentMethod), result.State.ResponseText)
}

func (s *TransactionsIntegrationSuite) TestResumeDispatcher_FullCycle_PaymentClarifyConfirmPersists() {
	userID := uuid.New()
	threadID := "+5511930000003"
	s.seedSuspendedExpenseAwaitingPayment(userID, threadID, "wamid-expense-start")

	dispatcher := s.buildTransactionWriteDispatcher()

	handled, reply, err := dispatcher.Continue(context.Background(), userID.String(), threadID, "paguei no pix", "wamid-pm")
	s.Require().NoError(err)
	s.True(handled)
	s.Contains(reply, "Posso registrar?")
	s.Contains(reply, "pix")
	s.Equal(0, s.countTransactions(userID))

	handled, reply, err = dispatcher.Continue(context.Background(), userID.String(), threadID, "sim", "wamid-sim")
	s.Require().NoError(err)
	s.True(handled)
	s.NotEmpty(reply)

	s.Equal(1, s.countTransactions(userID))
	s.Equal(1, s.countWriteLedger(userID))
}

func (s *TransactionsIntegrationSuite) TestResumeDispatcher_InvalidPaymentAnswer_RepromptThenCancel() {
	userID := uuid.New()
	threadID := "+5511930000004"
	s.seedSuspendedExpenseAwaitingPayment(userID, threadID, "wamid-expense-start-2")

	dispatcher := s.buildTransactionWriteDispatcher()

	handled, reply, err := dispatcher.Continue(context.Background(), userID.String(), threadID, "Sim", "wamid-invalid-1")
	s.Require().NoError(err)
	s.True(handled)
	s.Equal(messages.PaymentMethodReprompt(), reply)

	handled, reply, err = dispatcher.Continue(context.Background(), userID.String(), threadID, "qualquer coisa", "wamid-invalid-2")
	s.Require().NoError(err)
	s.True(handled)
	s.NotEmpty(reply)

	s.Equal(0, s.countTransactions(userID))
	s.Equal(0, s.countWriteLedger(userID))
}

func (s *TransactionsIntegrationSuite) TestResumeDispatcher_ReplayedWamid_NoDuplicateWrite() {
	userID := uuid.New()
	threadID := "+5511930000005"
	s.seedSuspendedIncomeConfirmation(userID, threadID, "wamid-income-start")

	dispatcher := s.buildTransactionWriteDispatcher()

	handled, reply, err := dispatcher.Continue(context.Background(), userID.String(), threadID, "talvez", "wamid-ambiguo")
	s.Require().NoError(err)
	s.True(handled)
	s.NotEmpty(reply)

	handled, replayReply, err := dispatcher.Continue(context.Background(), userID.String(), threadID, "talvez", "wamid-ambiguo")
	s.Require().NoError(err)
	s.True(handled)
	s.Equal(reply, replayReply)
	s.Equal(0, s.countTransactions(userID))

	handled, _, err = dispatcher.Continue(context.Background(), userID.String(), threadID, "sim", "wamid-sim-final")
	s.Require().NoError(err)
	s.True(handled)
	s.Equal(1, s.countTransactions(userID))
	s.Equal(1, s.countWriteLedger(userID))
}
