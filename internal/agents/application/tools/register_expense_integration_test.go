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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	mempostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type stubLedger struct {
	createdID uuid.UUID
}

func (s *stubLedger) CreateTransaction(_ context.Context, _ interfaces.RawTransaction) (interfaces.EntryRef, error) {
	return interfaces.EntryRef{ID: s.createdID, Kind: "transaction", Reconciled: false}, nil
}

func (s *stubLedger) CreateCardPurchase(_ context.Context, _ interfaces.RawCardPurchase) (interfaces.EntryRef, error) {
	return interfaces.EntryRef{}, nil
}

func (s *stubLedger) UpdateTransaction(_ context.Context, _ interfaces.RawUpdateTransaction) (interfaces.EntryRef, error) {
	return interfaces.EntryRef{}, nil
}

func (s *stubLedger) DeleteTransaction(_ context.Context, _ interfaces.EntryRef, _ int64) error {
	return nil
}

func (s *stubLedger) UpdateCardPurchase(_ context.Context, _ interfaces.RawUpdateCardPurchase) (interfaces.EntryRef, error) {
	return interfaces.EntryRef{}, nil
}

func (s *stubLedger) DeleteCardPurchase(_ context.Context, _ interfaces.EntryRef, _ int64) error {
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

func (s *stubLedger) GetCardPurchase(_ context.Context, _ uuid.UUID) (interfaces.Entry, error) {
	return interfaces.Entry{}, nil
}

func (s *stubLedger) ListCardPurchases(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]interfaces.Entry, error) {
	return nil, nil
}

type stubCategoriesReader struct {
	rootID uuid.UUID
	leafID uuid.UUID
}

func (r *stubCategoriesReader) SearchDictionary(_ context.Context, _, _ string) ([]interfaces.CategoryCandidate, error) {
	return []interfaces.CategoryCandidate{
		{CategoryID: r.leafID, RootCategoryID: r.rootID, Path: "Alimentação > Restaurante", Score: 0.95},
	}, nil
}

func (r *stubCategoriesReader) ResolveRootsBySlug(_ context.Context, _ []string) (map[string]uuid.UUID, error) {
	return map[string]uuid.UUID{}, nil
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

func (s *RegisterExpenseIntegrationSuite) TestIdentityInjectedAndWrittenToLedger() {
	obs := fake.NewProvider()
	repo := persistence.NewWriteLedgerRepository(s.db, obs)
	writer := usecases.NewIdempotentWrite(repo, obs)
	reader := &stubCategoriesReader{rootID: uuid.New(), leafID: uuid.New()}
	registrar := usecases.NewRegisterEntry(reader, &stubLedger{createdID: uuid.New()}, writer, obs)
	handle := BuildRegisterExpenseTool(registrar)

	agentID := "agent-expense-" + uuid.NewString()
	ag := &toolInvokingAgent{id: agentID, handle: handle}
	reg := agent.NewAgentRegistry()
	reg.Register(ag)

	userID := uuid.New()
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
		ThreadID:   "thr-" + uuid.NewString(),
		Message:    "registrar despesa",
		MessageID:  wamid,
	})
	s.Require().NoError(err)
	s.Equal(agent.RunStatusSucceeded, outcome.Status)

	var ledgerCount int
	err = s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM mecontrola.agents_write_ledger WHERE wamid = $1 AND item_seq = 0 AND operation = 'create_expense'`,
		wamid,
	).Scan(&ledgerCount)
	s.Require().NoError(err)
	s.Greater(ledgerCount, 0, "RF-37/EP-01: agents_write_ledger deve conter linha com identidade injetada server-side")

	var toolMsgCount int
	err = s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM mecontrola.platform_messages WHERE resource_id = $1 AND role = $2`,
		userID.String(), string(memory.RoleTool),
	).Scan(&toolMsgCount)
	s.Require().NoError(err)
	s.Equal(0, toolMsgCount, "RF-39: mensagens role=tool NÃO devem ser persistidas no histórico (evita órfão tool → HTTP 400)")
}
