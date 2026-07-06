//go:build integration

package agents

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	agenttools "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/tools"
	agentusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/binding"
	agentpersistence "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
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

func (h *e2eSpyHooks) AfterTool(_ context.Context, _, toolID string, resultBytes []byte, err error) {
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
	return agentsifaces.CategoryWriteDecision{}, nil
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
		createTx, nil, nil, listME, getMS, nil, nil, nil, o11y,
	)

	s.ledgerRepo = agentpersistence.NewWriteLedgerRepository(db, o11y)
	s.idem = agentusecases.NewIdempotentWrite(s.ledgerRepo, o11y)

	reader := &e2eStubCategoriesReader{rootID: s.categoryID, leafID: s.subcategoryID, version: s.editorialVersion}
	registerEntry := agentusecases.NewRegisterEntry(reader, s.adapter, s.idem, o11y)

	s.tools = []tool.ToolHandle{
		agenttools.BuildRegisterExpenseTool(registerEntry),
		agenttools.BuildQueryMonthTool(s.adapter),
	}
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
	a := BuildMeControlaAgent(s.provider, s.tools, hooks, fake.NewProvider())

	ctx, cancel := context.WithTimeout(s.authedCtx(), 90*time.Second)
	defer cancel()

	result, err := a.Execute(ctx, agent.Request{
		AgentID: MecontrolaAgentID,
		Messages: []llm.Message{
			{Role: "user", Content: "Registre a despesa de almoço de 50 reais no débito. Não peça confirmação, apenas registre."},
		},
		MaxTokens: 512,
	})
	s.Require().NoError(err)
	s.T().Logf("resposta do agente E2E-1: %s", result.Content)
	s.T().Logf("tool calls=%d tool errs=%d lastTool=%s", hooks.calls, hooks.errs, hooks.lastID)

	s.Require().Equal(1, s.countTransactions(),
		"deve existir exatamente 1 transação persistida para o usuário após o registro via LLM")
	s.Require().Equal(1, s.countTransactionsWithCategory(),
		"a transação persistida deve ter categoria resolvida deterministicamente (category_id não nulo)")

	wamid, itemSeq, operation, resourceID, found := s.findLedgerRow()
	s.Require().True(found, "deve existir uma entrada no write ledger para o usuário")
	s.Equal("create_expense", operation, "operação do ledger deve ser create_expense")

	ledgerEntry, err := s.ledgerRepo.FindByKey(s.ctx, wamid, itemSeq, operation)
	s.Require().NoError(err, "ledger deve ser localizável pela chave real usada pelo agente")
	s.Equal(resourceID, ledgerEntry.ResourceID)

	s.firstWamid = wamid
	s.firstSeq = itemSeq
	s.firstOpName = operation
	s.firstTxID = resourceID
}

func (s *MeControlaAgentE2ESuite) TestE2E2_ReprocessarMesmoWamidNaoDuplica() {
	s.Require().NotEmpty(s.firstWamid, "E2E1 deve ter registrado a despesa e populado a chave do ledger")

	before := s.countTransactions()
	s.Require().Equal(1, before, "estado inicial deve ter exatamente 1 transação")

	res, err := s.idem.Execute(s.authedCtx(), s.userID, s.firstWamid, s.firstSeq, s.firstOpName, "transaction",
		func(innerCtx context.Context) (uuid.UUID, bool, error) {
			ref, createErr := s.adapter.CreateTransaction(innerCtx, agentsifaces.RawTransaction{
				Direction:     "outcome",
				PaymentMethod: "debit",
				AmountCents:   5000,
				Description:   "almoço duplicado",
				CategoryID:    s.categoryID,
				SubcategoryID: &s.subcategoryID,
				OccurredAt:    time.Now().Format("2006-01-02"),
			})
			if createErr != nil {
				return uuid.Nil, false, createErr
			}
			return ref.ID, ref.Reconciled, nil
		},
	)
	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeReplay, res.Outcome,
		"reexecutar com o mesmo wamid+itemSeq+operation deve ser replay, sem nova escrita")
	s.Equal(s.firstTxID, res.ResourceID, "replay deve retornar o resourceID original")

	s.Equal(1, s.countTransactions(),
		"não deve duplicar a transação após replay idempotente ponta-a-ponta")
}
