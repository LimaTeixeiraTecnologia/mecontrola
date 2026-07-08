//go:build integration

package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	mempostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type stubLedger struct {
	createdID uuid.UUID
}

func (s *stubLedger) CreateTransaction(_ context.Context, _ interfaces.RawTransaction) (interfaces.EntryRef, error) {
	return interfaces.EntryRef{ID: s.createdID, Kind: interfaces.EntryKindTransaction, Reconciled: false}, nil
}

func (s *stubLedger) UpdateTransaction(_ context.Context, _ interfaces.RawUpdateTransaction) (interfaces.EntryRef, error) {
	return interfaces.EntryRef{}, nil
}

func (s *stubLedger) DeleteTransaction(_ context.Context, _ interfaces.EntryRef, _ int64) error {
	return nil
}

func (s *stubLedger) ListMonthlyEntries(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]interfaces.MonthlyEntry, error) {
	return nil, nil
}

func (s *stubLedger) GetMonthlySummary(_ context.Context, _ uuid.UUID, _ string) (interfaces.MonthlySummary, error) {
	return interfaces.MonthlySummary{}, nil
}

func (s *stubLedger) GetCardInvoice(_ context.Context, _ uuid.UUID, _ string) (interfaces.CardInvoice, error) {
	return interfaces.CardInvoice{}, nil
}

func (s *stubLedger) SearchTransactions(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]interfaces.Entry, error) {
	return nil, nil
}

func (s *stubLedger) GetTransaction(_ context.Context, _ string) (interfaces.Entry, error) {
	return interfaces.Entry{}, nil
}

func (s *stubLedger) CreateRecurringTemplate(_ context.Context, _ interfaces.RawRecurringTemplate) (interfaces.EntryRef, error) {
	return interfaces.EntryRef{}, nil
}

type stubCategoriesReader struct {
	rootID uuid.UUID
	leafID uuid.UUID
}

func (r *stubCategoriesReader) SearchDictionary(_ context.Context, _, _ string) (interfaces.CategorySearchResult, error) {
	return interfaces.CategorySearchResult{
		Outcome: interfaces.ClassifyOutcomeMatched,
		Version: 1,
		Candidates: []interfaces.CategoryCandidate{
			{CategoryID: r.leafID, RootCategoryID: r.rootID, Path: "Alimentação > Restaurante", Score: 0.95, SignalType: "alias", Confidence: "high", MatchQuality: "exact", MatchReason: "alias match"},
		},
	}, nil
}

func (r *stubCategoriesReader) ResolveForWrite(_ context.Context, _ interfaces.CategoryWriteRequest) (interfaces.CategoryWriteDecision, error) {
	return interfaces.CategoryWriteDecision{}, nil
}

func (r *stubCategoriesReader) ListCategories(_ context.Context, _ uuid.UUID) ([]interfaces.Category, error) {
	return nil, nil
}

type toolInvokingAgent struct {
	id     string
	handle tool.ToolHandle
}

func (a *toolInvokingAgent) ID() string           { return a.id }
func (a *toolInvokingAgent) Instructions() string { return "test" }

func (a *toolInvokingAgent) Execute(ctx context.Context, _ agent.Request) (agent.Result, error) {
	args, _ := json.Marshal(RegisterExpenseInput{
		AmountCents:   5000,
		Description:   "Almoço",
		PaymentMethod: "debit_card",
		OccurredAt:    "2026-07-02",
	})
	raw, err := a.handle.Invoke(ctx, args)
	if err != nil {
		return agent.Result{}, err
	}
	return agent.Result{
		Content: "registrado",
		ToolCalls: []agent.ToolCallRecord{
			{Tool: a.handle.ID(), Outcome: agent.ToolCallOutcomeSuccess, Content: string(raw)},
		},
	}, nil
}

func (a *toolInvokingAgent) Stream(_ context.Context, _ agent.Request) (agent.ResultStream, error) {
	return nil, nil
}

type RegisterExpenseIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
}

func TestRegisterExpenseIntegrationSuite(t *testing.T) {
	suite.Run(t, new(RegisterExpenseIntegrationSuite))
}

func (s *RegisterExpenseIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
}

func (s *RegisterExpenseIntegrationSuite) TestIdentityInjectedAndPendingOpened() {
	obs := fake.NewProvider()
	reader := &stubCategoriesReader{rootID: uuid.New(), leafID: uuid.New()}
	store := workflowpg.NewPostgresStore(obs, s.db)
	engine := workflow.NewEngine[workflows.PendingEntryState](store, obs)
	def := workflows.BuildPendingEntryWorkflow(&stubLedger{createdID: uuid.New()}, nil, reader, nil)
	registrar := usecases.NewRegisterAttempt(reader, &stubLedger{createdID: uuid.New()}, engine, def, obs)
	handle := BuildRegisterExpenseTool(registrar)

	agentID := "agent-expense-" + uuid.NewString()
	ag := &toolInvokingAgent{id: agentID, handle: handle}
	reg := agent.NewAgentRegistry()
	reg.Register(ag)

	userID := uuid.New()
	threadID := "thr-" + uuid.NewString()
	wamid := "wamid-" + uuid.NewString()

	rt := agent.NewAgentRuntime(
		reg,
		mempostgres.NewThreadRepository(s.db, obs),
		mempostgres.NewMessageRepository(s.db, obs),
		mempostgres.NewWorkingMemoryRepository(s.db, obs),
		agentpostgres.NewRunStore(s.db),
		obs,
		agent.WithWriteToolSet(handle.ID()),
	)

	outcome, err := rt.Execute(s.ctx, agent.InboundRequest{
		AgentID:    agentID,
		ResourceID: userID.String(),
		ThreadID:   threadID,
		Message:    "registrar despesa",
		MessageID:  wamid,
	})
	s.Require().NoError(err)
	s.Equal(agent.RunStatusSucceeded, outcome.Status)

	key := workflows.PendingEntryKey(userID.String(), threadID)
	snap, found, loadErr := store.Load(s.ctx, workflows.PendingEntryWorkflowID, key)
	s.Require().NoError(loadErr)
	s.Require().True(found, "RF-38/O-07: register deve abrir pendência durável (gate de confirmação), não escrever de forma síncrona")
	s.Equal(workflow.RunStatusSuspended, snap.Status)

	var toolMsgCount int
	err = s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM mecontrola.platform_messages WHERE resource_id = $1 AND role = $2`,
		userID.String(), string(memory.RoleTool),
	).Scan(&toolMsgCount)
	s.Require().NoError(err)
	s.Equal(0, toolMsgCount, "RF-39: mensagens role=tool NÃO devem ser persistidas no histórico (evita órfão tool → HTTP 400)")
}
