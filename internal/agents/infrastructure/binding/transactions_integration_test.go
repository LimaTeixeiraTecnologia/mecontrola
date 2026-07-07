//go:build integration

package binding_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	agentusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/binding"
	agentpersistence "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
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
	return txifaces.CategorySnapshot{ID: v.catID, Name: "Alimentação"}, nil
}

type stubCardLookup struct {
	snapshot valueobjects.CardBillingSnapshot
}

func (l *stubCardLookup) GetForUser(_ context.Context, _, _ uuid.UUID) (valueobjects.CardBillingSnapshot, error) {
	return l.snapshot, nil
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

type TransactionsIntegrationSuite struct {
	suite.Suite
	ctx           context.Context
	db            database.DBTX
	cardID        uuid.UUID
	cardOwner     uuid.UUID
	rootCatID     uuid.UUID
	leafCatID     uuid.UUID
	expenseRootID uuid.UUID
	expenseLeafID uuid.UUID
	adapter       agentsifaces.TransactionsLedger
	recompute     *txusecases.RecomputeMonthlySummary
	ledgerRepo    agentusecases.WriteLedgerRepository
	idemUC        *agentusecases.IdempotentWrite
}

func TestTransactionsIntegrationSuite(t *testing.T) {
	suite.Run(t, new(TransactionsIntegrationSuite))
}

func (s *TransactionsIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.cardID = uuid.New()
	s.cardOwner = uuid.New()

	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	o11y := fake.NewProvider()
	factory := txrepos.NewRepositoryFactory(o11y)
	s.rootCatID = uuid.New()
	s.leafCatID = uuid.New()
	s.expenseRootID = uuid.New()
	s.expenseLeafID = uuid.New()
	catID := s.leafCatID

	_, err := db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type)
		VALUES ($1, 'integ-salario', 'Salário', 'income', NULL, 'consumption'),
		       ($2, 'integ-bonus', 'Bônus', 'income', $1, 'consumption'),
		       ($3, 'integ-alimentacao', 'Alimentação', 'expense', NULL, 'consumption'),
		       ($4, 'integ-restaurante', 'Restaurante', 'expense', $3, 'consumption')`,
		s.rootCatID, s.leafCatID, s.expenseRootID, s.expenseLeafID,
	)
	s.Require().NoError(err)

	var editorialVersion int64
	s.Require().NoError(
		db.QueryRowContext(s.ctx, `SELECT version FROM mecontrola.category_editorial_version LIMIT 1`).Scan(&editorialVersion),
	)

	_, err = db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511999990001', 'ACTIVE', now(), now())`,
		s.cardOwner,
	)
	s.Require().NoError(err)

	_, err = db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, nickname, bank, closing_day, due_day, version, created_at, updated_at)
		VALUES ($1, $2, 'nubank', 'nubank', 20, 25, 1, now(), now())`,
		s.cardID, s.cardOwner,
	)
	s.Require().NoError(err)

	snapshot, err := valueobjects.NewCardBillingSnapshot(20, 25)
	s.Require().NoError(err)

	createTx := txusecases.NewCreateTransaction(
		factory,
		uow.NewUnitOfWork(db),
		&stubCardLookup{snapshot: snapshot},
		&stubCategoryValidator{catID: catID},
		&stubCategoryWriteGate{version: editorialVersion},
		services.TransactionWorkflow{},
		&noopTxPublisher{},
		o11y,
	)

	getMS := txusecases.NewGetMonthlySummary(factory, uow.NewUnitOfWork(db), o11y)
	listME := txusecases.NewListMonthlyEntries(factory, uow.NewUnitOfWork(db), o11y)

	s.adapter = binding.NewTransactionsLedgerAdapter(
		createTx, nil, nil, listME, getMS, nil, nil, nil, nil, o11y,
	)

	s.recompute = txusecases.NewRecomputeMonthlySummary(factory, uow.NewUnitOfWork(db), o11y)
	s.ledgerRepo = agentpersistence.NewWriteLedgerRepository(db, o11y)
	s.idemUC = agentusecases.NewIdempotentWrite(s.ledgerRepo, o11y)
}

func (s *TransactionsIntegrationSuite) authedCtx(userID uuid.UUID) context.Context {
	return auth.WithPrincipal(s.ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
}

func (s *TransactionsIntegrationSuite) doRecompute(userID uuid.UUID, refMonth string) {
	rm, err := valueobjects.NewRefMonth(refMonth)
	s.Require().NoError(err)
	s.Require().NoError(s.recompute.Execute(s.authedCtx(userID), txusecases.RecomputeMonthlySummaryInput{
		UserID:   userID,
		RefMonth: rm,
	}))
}

func (s *TransactionsIntegrationSuite) TestCenario1_ExpensaRefleteNoResumoSemDuplaContagem() {
	userID := uuid.New()
	ctx := s.authedCtx(userID)

	ref, err := s.adapter.CreateTransaction(ctx, agentsifaces.RawTransaction{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   5000,
		Description:   "Salário teste",
		CategoryID:    s.rootCatID,
		SubcategoryID: &s.leafCatID,
		OccurredAt:    "2026-07-01",
	})
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, ref.ID)
	s.Equal(agentsifaces.EntryKindTransaction, ref.Kind)

	s.doRecompute(userID, "2026-07")

	summary, err := s.adapter.GetMonthlySummary(ctx, userID, "2026-07")
	s.Require().NoError(err)
	s.Equal(int64(5000), summary.IncomeCents, "receita de 5000 deve aparecer no resumo")

	s.doRecompute(userID, "2026-07")

	summary2, err := s.adapter.GetMonthlySummary(ctx, userID, "2026-07")
	s.Require().NoError(err)
	s.Equal(summary.IncomeCents, summary2.IncomeCents, "recalcular duas vezes não deve duplicar o valor")
}

func (s *TransactionsIntegrationSuite) TestCenario2_IdempotenciaMesmoWamidNaoDuplica() {
	userID := uuid.New()
	ctx := s.authedCtx(userID)
	wamid := "wamid-idem-" + uuid.NewString()

	var firstID uuid.UUID
	result1, err := s.idemUC.Execute(ctx, userID, wamid, 0, "create_transaction", "transaction",
		func(innerCtx context.Context) (uuid.UUID, bool, error) {
			ref, createErr := s.adapter.CreateTransaction(innerCtx, agentsifaces.RawTransaction{
				Direction:     "income",
				PaymentMethod: "pix",
				AmountCents:   3000,
				Description:   "Renda idempotente",
				CategoryID:    s.rootCatID,
				SubcategoryID: &s.leafCatID,
				OccurredAt:    "2026-07-01",
			})
			if createErr != nil {
				return uuid.Nil, false, createErr
			}
			firstID = ref.ID
			return ref.ID, ref.Reconciled, nil
		},
	)
	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeRouted, result1.Outcome)
	s.Equal(firstID, result1.ResourceID)

	result2, err := s.idemUC.Execute(ctx, userID, wamid, 0, "create_transaction", "transaction",
		func(innerCtx context.Context) (uuid.UUID, bool, error) {
			ref, createErr := s.adapter.CreateTransaction(innerCtx, agentsifaces.RawTransaction{
				Direction:     "income",
				PaymentMethod: "pix",
				AmountCents:   3000,
				Description:   "Renda idempotente",
				CategoryID:    s.rootCatID,
				SubcategoryID: &s.leafCatID,
				OccurredAt:    "2026-07-01",
			})
			if createErr != nil {
				return uuid.Nil, false, createErr
			}
			return ref.ID, ref.Reconciled, nil
		},
	)
	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeReplay, result2.Outcome, "segunda chamada com mesmo wamid deve ser replay")
	s.Equal(firstID, result2.ResourceID, "replay deve retornar o mesmo resourceID")

	found, err := s.ledgerRepo.FindByKey(ctx, wamid, 0, "create_transaction")
	s.Require().NoError(err)
	s.Equal(firstID, found.ResourceID, "ledger deve ter exatamente uma entrada para o wamid")

	s.doRecompute(userID, "2026-07")

	summary, err := s.adapter.GetMonthlySummary(ctx, userID, "2026-07")
	s.Require().NoError(err)
	s.Equal(int64(3000), summary.IncomeCents, "resumo deve refletir apenas um lançamento, sem dupla contagem")
}

func (s *TransactionsIntegrationSuite) TestOriginRef_BindingReprocess_NoDuplicate() {
	userID := uuid.New()
	ctx := s.authedCtx(userID)
	wamid := "wamid-m1"

	raw := agentsifaces.RawTransaction{
		Direction:       "income",
		PaymentMethod:   "pix",
		AmountCents:     4200,
		Description:     "Renda reentregue",
		CategoryID:      s.rootCatID,
		SubcategoryID:   &s.leafCatID,
		OccurredAt:      "2026-07-01",
		OriginWamid:     wamid,
		OriginItemSeq:   0,
		OriginOperation: "create_income",
	}

	ref1, err := s.adapter.CreateTransaction(ctx, raw)
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, ref1.ID)
	s.False(ref1.Reconciled, "primeira escrita cria a linha, não reconcilia")

	ref2, err := s.adapter.CreateTransaction(ctx, raw)
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, ref2.ID)
	s.True(ref2.Reconciled, "segunda escrita com mesmo OriginRef reconcilia via ON CONFLICT")

	s.Equal(ref1.ID, ref2.ID, "reprocessamento com mesmo OriginRef deve retornar a mesma transação")

	var count int
	err = s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM mecontrola.transactions WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count, "idempotência de domínio deve manter exatamente uma linha para o OriginRef")
}

func (s *TransactionsIntegrationSuite) TestCenario3_CartaoParceladoRefleteSoUmaParcela() {
	userID := s.cardOwner
	ctx := s.authedCtx(userID)

	cardID := s.cardID
	ref, err := s.adapter.CreateTransaction(ctx, agentsifaces.RawTransaction{
		Direction:     "outcome",
		PaymentMethod: "credit_card",
		AmountCents:   30000,
		Description:   "Eletrodoméstico parcelado",
		CategoryID:    s.expenseRootID,
		SubcategoryID: &s.expenseLeafID,
		CardID:        &cardID,
		Installments:  3,
		OccurredAt:    "2026-07-01",
	})
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, ref.ID)
	s.Equal(agentsifaces.EntryKindTransaction, ref.Kind)

	s.doRecompute(userID, "2026-07")

	summary, err := s.adapter.GetMonthlySummary(ctx, userID, "2026-07")
	s.Require().NoError(err)

	s.Less(summary.OutcomeCents, int64(30000),
		"apenas uma parcela deve aparecer no mês de compra, não o total de 30000")
	s.GreaterOrEqual(summary.OutcomeCents, int64(9000),
		"a parcela do mês deve estar refletida no resumo")

	s.doRecompute(userID, "2026-07")

	summary2, err := s.adapter.GetMonthlySummary(ctx, userID, "2026-07")
	s.Require().NoError(err)
	s.Equal(summary.OutcomeCents, summary2.OutcomeCents, "recalcular não deve duplicar a parcela")
}
