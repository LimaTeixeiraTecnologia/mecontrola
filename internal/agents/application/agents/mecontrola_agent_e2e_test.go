//go:build integration

package agents

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	agenttools "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/tools"
	agentusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/binding"
	agentpersistence "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
	txifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	txusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	txrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories"
	"github.com/jmoiron/sqlx"
)

type e2eTxPublisher struct{}

func (p *e2eTxPublisher) PublishCreated(_ context.Context, _ database.DBTX, _ entities.TransactionCreated) error {
	return nil
}

func (p *e2eTxPublisher) PublishUpdated(_ context.Context, _ database.DBTX, _ entities.TransactionUpdated) error {
	return nil
}

func (p *e2eTxPublisher) PublishDeleted(_ context.Context, _ database.DBTX, _ entities.TransactionDeleted) error {
	return nil
}

type e2eSpyHooks struct {
	t      *testing.T
	calls  int
	errs   int
	lastID string
}

func (h *e2eSpyHooks) BeforeExecute(ctx context.Context, _ string, _ agent.Request) context.Context {
	return ctx
}

func (h *e2eSpyHooks) AfterExecute(_ context.Context, _ string, _ agent.Result, _ error) {}

func (h *e2eSpyHooks) BeforeTool(ctx context.Context, _, toolID string) context.Context {
	h.calls++
	h.lastID = toolID
	return ctx
}

func (h *e2eSpyHooks) AfterTool(_ context.Context, _, toolID string, _, resultBytes []byte, err error) {
	if err != nil {
		h.errs++
	}
	h.t.Logf("tool=%s err=%v result=%s", toolID, err, string(resultBytes))
}

type e2eStubCategoryValidator struct{ catID uuid.UUID }

func (v *e2eStubCategoryValidator) Validate(_ context.Context, _ uuid.UUID, _ *uuid.UUID) (txifaces.CategorySnapshot, error) {
	return txifaces.CategorySnapshot{ID: v.catID, Name: "Alimentação"}, nil
}

type e2eStubCardLookup struct {
	snapshot valueobjects.CardBillingSnapshot
}

func (l *e2eStubCardLookup) GetForUser(_ context.Context, _, _ uuid.UUID) (valueobjects.CardBillingSnapshot, error) {
	return l.snapshot, nil
}

type e2eStubCardManager struct{}

func (m *e2eStubCardManager) CreateCard(_ context.Context, _ agentsifaces.NewCard) (agentsifaces.CardRef, error) {
	return agentsifaces.CardRef{}, nil
}

func (m *e2eStubCardManager) ListCards(_ context.Context, _ uuid.UUID) ([]agentsifaces.Card, error) {
	return nil, nil
}

func (m *e2eStubCardManager) GetCard(_ context.Context, _, _ uuid.UUID) (agentsifaces.Card, error) {
	return agentsifaces.Card{}, agentsifaces.ErrCardNotFound
}

func (m *e2eStubCardManager) ResolveCardByNickname(_ context.Context, _ uuid.UUID, _ string) (agentsifaces.Card, error) {
	return agentsifaces.Card{}, agentsifaces.ErrCardNotFound
}

func (m *e2eStubCardManager) CountCards(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

func (m *e2eStubCardManager) BestPurchaseDay(_ context.Context, _ string, _ int) (agentsifaces.BestPurchaseDay, error) {
	return agentsifaces.BestPurchaseDay{}, nil
}

func (m *e2eStubCardManager) BankRecognized(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *e2eStubCardManager) UpdateCard(_ context.Context, _ agentsifaces.CardUpdate) (agentsifaces.Card, error) {
	return agentsifaces.Card{}, nil
}

func (m *e2eStubCardManager) SoftDeleteCard(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (m *e2eStubCardManager) HasOpenInstallments(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

type e2eStubCategoryGate struct{ version int64 }

func (g *e2eStubCategoryGate) Approve(_ context.Context, in txifaces.CategoryWriteGateInput) (valueobjects.CategoryWriteEvidence, error) {
	kind := "expense"
	if in.Direction == "income" {
		kind = "income"
	}
	return valueobjects.NewCategoryWriteEvidence(valueobjects.CategoryWriteEvidenceInput{
		RootCategoryID:   in.RootCategoryID,
		SubcategoryID:    in.SubcategoryID,
		Kind:             kind,
		Path:             "Alimentação > Restaurante",
		Outcome:          "matched",
		Score:            0.95,
		Confidence:       "high",
		Quality:          "exact",
		SignalType:       "alias",
		MatchedTerm:      "almoço",
		MatchReason:      "alias match",
		Source:           valueobjects.CategoryDecisionSourceAutoMatched,
		EditorialVersion: g.version,
		DecidedAt:        time.Now().UTC(),
	})
}

type e2eStubCategoriesReader struct {
	rootID  uuid.UUID
	leafID  uuid.UUID
	version int64
}

func (r *e2eStubCategoriesReader) SearchDictionary(_ context.Context, _, _ string) (agentsifaces.CategorySearchResult, error) {
	return agentsifaces.CategorySearchResult{
		Outcome: agentsifaces.ClassifyOutcomeMatched,
		Version: r.version,
		Candidates: []agentsifaces.CategoryCandidate{
			{CategoryID: r.leafID, RootCategoryID: r.rootID, Path: "Alimentação > Restaurante", Score: 0.95, SignalType: "alias", Confidence: "high", MatchQuality: "exact", MatchReason: "alias match"},
		},
	}, nil
}

func (r *e2eStubCategoriesReader) ResolveForWrite(_ context.Context, _ agentsifaces.CategoryWriteRequest) (agentsifaces.CategoryWriteDecision, error) {
	return agentsifaces.CategoryWriteDecision{
		RootCategoryID:   r.rootID,
		SubcategoryID:    r.leafID,
		Kind:             agentsifaces.CategoryKindExpense,
		Path:             "Alimentação > Restaurante",
		RootSlug:         "e2e-alimentacao",
		SubcategorySlug:  "e2e-restaurante",
		EditorialVersion: r.version,
	}, nil
}

func (r *e2eStubCategoriesReader) ListCategories(_ context.Context, _ uuid.UUID) ([]agentsifaces.Category, error) {
	return nil, nil
}

type MeControlaAgentE2ESuite struct {
	suite.Suite
	ctx              context.Context
	db               *sqlx.DB
	userID           uuid.UUID
	categoryID       uuid.UUID
	subcategoryID    uuid.UUID
	editorialVersion int64
	adapter          agentsifaces.TransactionsLedger
	ledgerRepo       agentusecases.WriteLedgerRepository
	idem             *agentusecases.IdempotentWrite
	provider         llm.Provider
	tools            []tool.ToolHandle
	pendingEngine    workflow.Engine[workflows.PendingEntryState]
	pendingDef       workflow.Definition[workflows.PendingEntryState]
	firstWamid       string
	firstSeq         int
	firstTxID        uuid.UUID
	firstOpName      string
}

func TestMeControlaAgentE2ESuite(t *testing.T) {
	suite.Run(t, new(MeControlaAgentE2ESuite))
}

func (s *MeControlaAgentE2ESuite) SetupSuite() {
	s.ctx = context.Background()
	s.provider = buildRealLLMProvider(s.T())

	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	o11y := fake.NewProvider()
	factory := txrepos.NewRepositoryFactory(o11y)

	s.userID = uuid.New()
	s.categoryID = uuid.New()
	s.subcategoryID = uuid.New()
	catID := uuid.New()

	_, err := db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511988880001', 'ACTIVE', now(), now())`,
		s.userID,
	)
	s.Require().NoError(err)

	_, err = db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type)
		VALUES ($1, 'e2e-alimentacao', 'Alimentação', 'expense', NULL, 'consumption'),
		       ($2, 'e2e-restaurante', 'Restaurante', 'expense', $1, 'consumption')`,
		s.categoryID, s.subcategoryID,
	)
	s.Require().NoError(err)

	s.Require().NoError(
		db.QueryRowContext(s.ctx, `SELECT version FROM mecontrola.category_editorial_version LIMIT 1`).Scan(&s.editorialVersion),
	)

	snapshot, err := valueobjects.NewCardBillingSnapshot(20, 25)
	s.Require().NoError(err)

	createTx := txusecases.NewCreateTransaction(
		factory,
		uow.NewUnitOfWork(db),
		&e2eStubCardLookup{snapshot: snapshot},
		&e2eStubCategoryValidator{catID: catID},
		&e2eStubCategoryGate{version: s.editorialVersion},
		services.TransactionWorkflow{},
		&e2eTxPublisher{},
		o11y,
	)

	getMS := txusecases.NewGetMonthlySummary(factory, uow.NewUnitOfWork(db), o11y)
	listME := txusecases.NewListMonthlyEntries(factory, uow.NewUnitOfWork(db), o11y)

	s.adapter = binding.NewTransactionsLedgerAdapter(
		createTx, nil, nil, listME, getMS, nil, nil, nil, nil, o11y,
	)

	s.ledgerRepo = agentpersistence.NewWriteLedgerRepository(db, o11y)
	s.idem = agentusecases.NewIdempotentWrite(s.ledgerRepo, o11y)

	reader := &e2eStubCategoriesReader{rootID: s.categoryID, leafID: s.subcategoryID, version: s.editorialVersion}
	store := workflowpg.NewPostgresStore(o11y, db)
	s.pendingEngine = workflow.NewEngine[workflows.PendingEntryState](store, o11y)
	s.pendingDef = workflows.BuildPendingEntryWorkflow(s.adapter, nil, reader, nil)
	registerAttempt := agentusecases.NewRegisterAttempt(reader, s.adapter, s.pendingEngine, s.pendingDef, o11y)

	s.tools = []tool.ToolHandle{
		agenttools.BuildRegisterExpenseTool(registerAttempt, &e2eStubCardManager{}),
		agenttools.BuildQueryMonthTool(s.adapter),
	}
}

func (s *MeControlaAgentE2ESuite) confirmPending() (workflow.RunResult[workflows.PendingEntryState], error) {
	key := workflows.PendingEntryKey(s.userID.String(), "")
	patch, _ := json.Marshal(map[string]string{"resumeText": "sim", "incomingMessageId": "wamid-e2e-confirm"})
	return s.pendingEngine.Resume(s.authedCtx(), s.pendingDef, key, patch)
}

func (s *MeControlaAgentE2ESuite) authedCtx() context.Context {
	ctx := auth.WithPrincipal(s.ctx, auth.Principal{UserID: s.userID, Source: auth.SourceWhatsApp})
	return agent.WithToolInvocationContext(ctx, s.userID.String(), "wamid-e2e-expense-1", 0)
}

func (s *MeControlaAgentE2ESuite) countTransactions() int {
	var n int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT count(*) FROM mecontrola.transactions WHERE user_id = $1 AND deleted_at IS NULL`,
		s.userID,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *MeControlaAgentE2ESuite) countTransactionsWithCategory() int {
	var n int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT count(*) FROM mecontrola.transactions WHERE user_id = $1 AND deleted_at IS NULL AND category_id IS NOT NULL`,
		s.userID,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *MeControlaAgentE2ESuite) findLedgerRow() (wamid string, itemSeq int, operation string, resourceID uuid.UUID, found bool) {
	err := s.db.QueryRowContext(s.ctx,
		`SELECT wamid, item_seq, operation, resource_id
		   FROM mecontrola.agents_write_ledger
		  WHERE user_id = $1
		  ORDER BY created_at DESC
		  LIMIT 1`,
		s.userID,
	).Scan(&wamid, &itemSeq, &operation, &resourceID)
	if err != nil {
		return "", 0, "", uuid.Nil, false
	}
	return wamid, itemSeq, operation, resourceID, true
}

func (s *MeControlaAgentE2ESuite) TestE2E1_RegistrarDespesaViaLLMPersisteNoBanco() {
	hooks := &e2eSpyHooks{t: s.T()}
	a := BuildMeControlaAgent(s.provider, s.tools, hooks, fake.NewProvider(), 0)

	ctx, cancel := context.WithTimeout(s.authedCtx(), 90*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Registre a despesa de almoço de 50 reais no débito."},
		},
		MaxTokens: 512,
	})
	s.Require().NoError(err)
	s.T().Logf("resposta do agente E2E-1: %s", result.Content)
	s.T().Logf("tool calls=%d tool errs=%d lastTool=%s", hooks.calls, hooks.errs, hooks.lastID)

	s.Require().Equal(0, s.countTransactions(),
		"O-07/RF-38: nenhuma transação deve ser persistida antes da confirmação humana explícita")

	runResult, confirmErr := s.confirmPending()
	s.Require().NoError(confirmErr)
	s.Equal(workflows.PendingStatusCompleted, runResult.State.Status)

	s.Require().Equal(1, s.countTransactions(),
		"deve existir exatamente 1 transação persistida após a confirmação explícita")
	s.Require().Equal(1, s.countTransactionsWithCategory(),
		"a transação persistida deve ter categoria resolvida deterministicamente (category_id não nulo)")
}

func (s *MeControlaAgentE2ESuite) TestE2E2_ReconfirmarNaoDuplica() {
	before := s.countTransactions()
	s.Require().Equal(1, before, "estado inicial deve ter exatamente 1 transação (confirmada em E2E1)")

	runResult, err := s.confirmPending()
	s.Require().NoError(err)
	s.Equal(workflow.RunStatus(0), runResult.Status,
		"CA-07: reconfirmar um run já concluído é no-op (idempotência pelo ciclo de vida do Run)")

	s.Equal(1, s.countTransactions(),
		"não deve duplicar a transação após reconfirmação idempotente ponta-a-ponta")
}
