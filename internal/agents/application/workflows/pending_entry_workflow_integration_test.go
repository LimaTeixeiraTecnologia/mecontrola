//go:build integration

package workflows_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/binding"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
	txifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	txusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	txrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories"
)

type noopTxPublisher struct{}

func (p *noopTxPublisher) PublishCreated(_ context.Context, _ database.DBTX, _ entities.TransactionCreated) error {
	return nil
}
func (p *noopTxPublisher) PublishUpdated(_ context.Context, _ database.DBTX, _ entities.TransactionUpdated) error {
	return nil
}
func (p *noopTxPublisher) PublishDeleted(_ context.Context, _ database.DBTX, _ entities.TransactionDeleted) error {
	return nil
}

type stubCategoryValidator struct{ catID uuid.UUID }

func (v *stubCategoryValidator) Validate(_ context.Context, _ uuid.UUID, _ *uuid.UUID) (txifaces.CategorySnapshot, error) {
	return txifaces.CategorySnapshot{ID: v.catID, Name: "Stub"}, nil
}

type stubCardLookup struct{}

func (l *stubCardLookup) GetForUser(_ context.Context, _, _ uuid.UUID) (valueobjects.CardBillingSnapshot, error) {
	return valueobjects.CardBillingSnapshot{}, nil
}

type stubCategoryWriteGate struct{ version int64 }

func (g *stubCategoryWriteGate) Approve(_ context.Context, in txifaces.CategoryWriteGateInput) (valueobjects.CategoryWriteEvidence, error) {
	kind := "income"
	if in.Direction == "outcome" {
		kind = "expense"
	}
	return valueobjects.ReconstituteEvidence(
		in.RootCategoryID,
		in.SubcategoryID,
		kind,
		"stub/categoria",
		"matched",
		1.0,
		"high",
		"exact",
		"canonical_name",
		"stub",
		"matched canonical_name stub",
		valueobjects.CategoryDecisionSourceAutoMatched,
		g.version,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	), nil
}

type realCategoriesReader struct {
	rootID uuid.UUID
	leafID uuid.UUID
	kind   agentsifaces.CategoryKind
}

func (r *realCategoriesReader) SearchDictionary(_ context.Context, _, _ string) (agentsifaces.CategorySearchResult, error) {
	return agentsifaces.CategorySearchResult{
		Outcome: agentsifaces.ClassifyOutcomeMatched,
		Version: 1,
		Candidates: []agentsifaces.CategoryCandidate{{
			CategoryID:     r.leafID,
			RootCategoryID: r.rootID,
			Path:           "Stub > Stub",
			Score:          0.95,
			Confidence:     "high",
			MatchQuality:   "exact",
			SignalType:     "canonical_name",
			MatchedTerm:    "stub",
		}},
	}, nil
}

func (r *realCategoriesReader) ResolveForWrite(_ context.Context, _ agentsifaces.CategoryWriteRequest) (agentsifaces.CategoryWriteDecision, error) {
	return agentsifaces.CategoryWriteDecision{
		RootCategoryID:   r.rootID,
		SubcategoryID:    r.leafID,
		RootSlug:         "stub",
		SubcategorySlug:  "stub",
		Path:             "Stub > Stub",
		EditorialVersion: 1,
	}, nil
}

func (r *realCategoriesReader) ListCategories(_ context.Context, _ uuid.UUID) ([]agentsifaces.Category, error) {
	return nil, nil
}

type PendingEntryWorkflowIntegrationSuite struct {
	suite.Suite
	ctx              context.Context
	db               *sqlx.DB
	ledger           agentsifaces.TransactionsLedger
	editorialVersion int64
}

func TestPendingEntryWorkflowIntegrationSuite(t *testing.T) {
	suite.Run(t, new(PendingEntryWorkflowIntegrationSuite))
}

func (s *PendingEntryWorkflowIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
	o11y := fake.NewProvider()
	factory := txrepos.NewRepositoryFactory(o11y)

	var editorialVersion int64
	s.Require().NoError(
		s.db.QueryRowContext(s.ctx, `SELECT version FROM mecontrola.category_editorial_version LIMIT 1`).Scan(&editorialVersion),
	)
	s.editorialVersion = editorialVersion

	createTx := txusecases.NewCreateTransaction(
		factory,
		uow.NewUnitOfWork(s.db),
		&stubCardLookup{},
		&stubCategoryValidator{},
		&stubCategoryWriteGate{version: editorialVersion},
		services.TransactionWorkflow{},
		&noopTxPublisher{},
		o11y,
	)
	getMS := txusecases.NewGetMonthlySummary(factory, uow.NewUnitOfWork(s.db), o11y)
	listME := txusecases.NewListMonthlyEntries(factory, uow.NewUnitOfWork(s.db), o11y)

	s.ledger = binding.NewTransactionsLedgerAdapter(
		createTx, nil, nil, listME, getMS, nil, nil, nil, nil, o11y,
	)
}

func (s *PendingEntryWorkflowIntegrationSuite) newUser() uuid.UUID {
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at) VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, "+55119"+uuid.NewString()[:8],
	)
	s.Require().NoError(err)
	return userID
}

func (s *PendingEntryWorkflowIntegrationSuite) createCategories(userID uuid.UUID, kind string) (rootID uuid.UUID, leafID uuid.UUID) {
	rootID = uuid.New()
	leafID = uuid.New()
	rootSlug := "integ-" + kind + "-root-" + uuid.NewString()
	leafSlug := "integ-" + kind + "-leaf-" + uuid.NewString()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type)
		VALUES ($1, $2, $3, $4, NULL, 'consumption'),
		       ($5, $6, $7, $4, $1, 'consumption')`,
		rootID, rootSlug, "Integ "+kind, kind,
		leafID, leafSlug, "Integ "+kind+" leaf",
	)
	s.Require().NoError(err)
	return rootID, leafID
}

func (s *PendingEntryWorkflowIntegrationSuite) authedCtx(userID uuid.UUID) context.Context {
	return auth.WithPrincipal(s.ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
}

func (s *PendingEntryWorkflowIntegrationSuite) buildEngine() (workflow.Engine[workflows.PendingEntryState], workflow.Definition[workflows.PendingEntryState]) {
	o11y := fake.NewProvider()
	store := workflowpg.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.PendingEntryState](store, o11y)
	def := workflows.BuildPendingEntryWorkflowWithObservability(s.ledger, nil, nil, nil, nil)
	return engine, def
}

func (s *PendingEntryWorkflowIntegrationSuite) TestRF39_DespesaPixConfirmada_PersisteEmTransactions() {
	userID := s.newUser()
	rootID, leafID := s.createCategories(userID, "expense")
	engine, def := s.buildEngine()

	key := workflows.PendingEntryKey(userID.String(), "thread-pix")
	state := workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      workflows.AwaitingSlotConfirmation,
		OperationKind: workflows.PendingOpRegisterExpense,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      "thread-pix",
		MessageID:     "wamid-pix-001",
		AmountCents:   5000,
		Description:   "supermercado",
		PaymentMethod: "pix",
		Kind:          agentsifaces.CategoryKindExpense,
		Candidates: []workflows.PendingCategoryCandidate{{
			RootCategoryID:  rootID,
			RootSlug:        "stub",
			SubcategoryID:   leafID,
			SubcategorySlug: "stub",
			Path:            "Stub > Stub",
		}},
		CategoryVersion: 1,
		OccurredAt:      time.Now().UTC().Format("2006-01-02"),
	}

	_, err := engine.Start(s.authedCtx(userID), def, key, state)
	s.Require().NoError(err)

	result, err := engine.Resume(s.authedCtx(userID), def, key, s.resumePayload("sim"))
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(workflows.PendingStatusCompleted, result.State.Status)

	var txID uuid.UUID
	var amountCents int64
	var description, originWamid string
	var paymentMethod int64
	scanErr := s.db.QueryRowContext(s.ctx, `
		SELECT id, amount_cents, description, payment_method, origin_wamid
		FROM mecontrola.transactions
		WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&txID, &amountCents, &description, &paymentMethod, &originWamid)
	s.Require().NoError(scanErr)
	s.NotEqual(uuid.Nil, txID)
	s.Equal(int64(5000), amountCents, "RF-39: amount_cents deve ser 5000")
	s.Equal("supermercado", description, "RF-39: description deve ser supermercado")
	s.Equal(int64(valueobjects.PaymentMethodPix), paymentMethod, "RF-39: payment_method deve ser pix")
	s.Equal("wamid-pix-001", originWamid, "RF-39: origin_wamid deve vir da pendência")
}

func (s *PendingEntryWorkflowIntegrationSuite) TestRF39_ReceitaSimplesConfirmada_PersisteEmTransactions() {
	userID := s.newUser()
	rootID, leafID := s.createCategories(userID, "income")
	engine, def := s.buildEngine()

	key := workflows.PendingEntryKey(userID.String(), "thread-income")
	state := workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      workflows.AwaitingSlotConfirmation,
		OperationKind: workflows.PendingOpRegisterIncome,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      "thread-income",
		MessageID:     "wamid-income-001",
		AmountCents:   1387440,
		Description:   "salário",
		PaymentMethod: "pix",
		Kind:          agentsifaces.CategoryKindIncome,
		Candidates: []workflows.PendingCategoryCandidate{{
			RootCategoryID:  rootID,
			RootSlug:        "stub",
			SubcategoryID:   leafID,
			SubcategorySlug: "stub",
			Path:            "Stub > Stub",
		}},
		CategoryVersion: 1,
		OccurredAt:      time.Now().UTC().Format("2006-01-02"),
	}

	_, err := engine.Start(s.authedCtx(userID), def, key, state)
	s.Require().NoError(err)

	result, err := engine.Resume(s.authedCtx(userID), def, key, s.resumePayload("sim"))
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSucceeded, result.Status)
	s.Equal(workflows.PendingStatusCompleted, result.State.Status)

	var txID uuid.UUID
	var amountCents int64
	var description string
	var direction int64
	scanErr := s.db.QueryRowContext(s.ctx, `
		SELECT id, amount_cents, description, direction
		FROM mecontrola.transactions
		WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&txID, &amountCents, &description, &direction)
	s.Require().NoError(scanErr)
	s.NotEqual(uuid.Nil, txID)
	s.Equal(int64(1387440), amountCents, "RF-39: amount_cents deve ser 1387440")
	s.Equal("salário", description, "RF-21/RF-39: description deve ser salário")
	s.Equal(int64(valueobjects.DirectionIncome), direction, "RF-39: direction deve ser income")
}

func (s *PendingEntryWorkflowIntegrationSuite) TestRF36_PixSemCartao_NaoExigeCartao() {
	userID := s.newUser()
	rootID, leafID := s.createCategories(userID, "expense")
	engine, def := s.buildEngine()

	key := workflows.PendingEntryKey(userID.String(), "thread-pix-no-card")
	state := workflows.PendingEntryState{
		Status:        workflows.PendingStatusActive,
		Awaiting:      workflows.AwaitingSlotPaymentMethod,
		OperationKind: workflows.PendingOpRegisterExpense,
		UserID:        userID,
		ResourceID:    userID,
		ThreadID:      "thread-pix-no-card",
		MessageID:     "wamid-pix-nc-001",
		AmountCents:   5000,
		Description:   "supermercado",
		Kind:          agentsifaces.CategoryKindExpense,
		Candidates: []workflows.PendingCategoryCandidate{{
			RootCategoryID:  rootID,
			RootSlug:        "stub",
			SubcategoryID:   leafID,
			SubcategorySlug: "stub",
			Path:            "Stub > Stub",
		}},
		CategoryVersion: 1,
	}

	startResult, err := engine.Start(s.authedCtx(userID), def, key, state)
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, startResult.Status)
	s.Equal(workflows.AwaitingSlotPaymentMethod, startResult.State.Awaiting)

	resumeResult, err := engine.Resume(s.authedCtx(userID), def, key, s.resumePayload("pix"))
	s.Require().NoError(err)
	s.Equal(workflow.RunStatusSuspended, resumeResult.Status)
	s.Equal(workflows.AwaitingSlotConfirmation, resumeResult.State.Awaiting)
	s.Equal("pix", resumeResult.State.PaymentMethod)
	s.Nil(resumeResult.State.CardID)
}

func (s *PendingEntryWorkflowIntegrationSuite) resumePayload(text string) []byte {
	b, _ := json.Marshal(map[string]string{"resumeText": text})
	return b
}
