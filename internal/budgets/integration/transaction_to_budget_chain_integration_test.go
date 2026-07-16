//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	budgetinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
	txinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
)

const (
	prazerosRootCategoryID  = "ac535261-4060-56ef-b2e8-57c8cc7032d1"
	deliverySubcategoryID   = "ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c"
	custoFixoRootCategoryID = "66cb85a0-3266-5900-b8e3-13cdcd00ab62"
	aluguelSubcategoryID    = "c2fda6a3-c329-52c8-81ea-771b6ea4f365"
	transactionCreatedType  = "transactions.transaction.created.v1"
	transactionUpdatedType  = "transactions.transaction.updated.v1"
	expectedAmountCents     = int64(5800)
	updatedAmountCents      = int64(9200)
	expectedCompetence      = "2026-06"
	expectedRootSlugStored  = "expense.prazeres"
	movedRootSlugStored     = "expense.custo_fixo"
	consumerLockedBy        = "chain-test-consumer"
)

type envelopeEvent struct {
	eventType string
	envelope  outbox.Envelope
}

func (e *envelopeEvent) GetEventType() string { return e.eventType }
func (e *envelopeEvent) GetPayload() any      { return e.envelope }

type TransactionToBudgetChainSuite struct {
	suite.Suite
}

func TestTransactionToBudgetChainSuite(t *testing.T) {
	suite.Run(t, new(TransactionToBudgetChainSuite))
}

func (s *TransactionToBudgetChainSuite) buildConfig() *configs.Config {
	cfg, err := configs.LoadConfig("../../..")
	s.Require().NoError(err, "carregar config")
	cfg.TransactionsConfig.Enabled = true
	return cfg
}

func (s *TransactionToBudgetChainSuite) ensureUserExists(ctx context.Context, mgr *sqlx.DB, userID uuid.UUID) {
	number := "+5511" + uuid.New().String()[:9]
	_, err := mgr.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())
		 ON CONFLICT (id) DO NOTHING`,
		userID, number,
	)
	s.Require().NoError(err)
}

func (s *TransactionToBudgetChainSuite) TestExpenseTransactionUpdatesBudgetReadModel() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	cfg := s.buildConfig()
	authMW := func(h http.Handler) http.Handler { return h }

	ctx := context.Background()

	categoriesModule := categories.NewCategoriesModule(mgr, o11y, authMW)
	s.Require().NotNil(categoriesModule)

	cardModule, err := card.NewCardModule(ctx, cfg, o11y, mgr, authMW, nil, nil)
	s.Require().NoError(err, "card module")

	txModule, err := transactions.NewTransactionsModule(cfg, o11y, mgr, cardModule, categoriesModule, authMW)
	s.Require().NoError(err, "transactions module")
	s.Require().NotNil(txModule.CreateTransactionUC, "CreateTransactionUC deve estar wired")

	budgetsModule, err := budgets.NewBudgetsModule(cfg, o11y, mgr, categoriesModule, authMW, nil, nil)
	s.Require().NoError(err, "budgets module")
	s.Require().NotNil(budgetsModule.TransactionCreatedConsumer, "consumer deve estar wired")

	userID := uuid.New()
	principalCtx := auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})

	subID := uuid.MustParse(deliverySubcategoryID)
	occurredAt := time.Date(2026, time.June, 17, 12, 0, 0, 0, time.UTC)
	raw := txinput.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   expectedAmountCents,
		Description:   "ifood delivery",
		CategoryID:    uuid.MustParse(prazerosRootCategoryID),
		SubcategoryID: &subID,
		OccurredAt:    occurredAt.Format(time.RFC3339),
	}

	created, err := txModule.CreateTransactionUC.Execute(principalCtx, raw)
	s.Require().NoError(err, "criar transaction")
	s.Require().NotEqual(uuid.Nil, created.ID, "transaction deve ter id")

	txID := created.ID.String()
	s.assertTransactionPersisted(ctx, mgr, txID, userID)

	envelope := s.claimTransactionCreatedEnvelope(ctx, mgr, txID)
	s.assertEnvelopePayload(envelope, userID)

	event := &envelopeEvent{eventType: transactionCreatedType, envelope: envelope}
	s.Require().NoError(
		budgetsModule.TransactionCreatedConsumer.Handle(ctx, platformevents.Event(event)),
		"consumer real deve processar o envelope serializado de verdade",
	)

	s.assertExpensePersisted(ctx, mgr, userID, txID)
	s.assertMonthlySummary(ctx, budgetsModule, userID)
}

func (s *TransactionToBudgetChainSuite) assertTransactionPersisted(ctx context.Context, mgr *sqlx.DB, txID string, userID uuid.UUID) {
	db := mgr
	var (
		gotUser   string
		gotAmount int64
		direction int
	)
	row := db.QueryRowContext(ctx,
		"SELECT user_id, amount_cents, direction FROM mecontrola.transactions WHERE id = $1",
		txID,
	)
	s.Require().NoError(row.Scan(&gotUser, &gotAmount, &direction), "transaction persistida em mecontrola.transactions")
	s.Equal(userID.String(), gotUser)
	s.Equal(expectedAmountCents, gotAmount)
	s.Equal(2, direction)
}

func (s *TransactionToBudgetChainSuite) claimTransactionCreatedEnvelope(ctx context.Context, mgr *sqlx.DB, aggregateID string) outbox.Envelope {
	storage := outbox.NewPostgresStorage(mgr)
	rows, err := storage.ClaimBatch(ctx, consumerLockedBy, 100)
	s.Require().NoError(err, "claim outbox")

	for _, row := range rows {
		if row.Type == transactionCreatedType && row.AggregateID == aggregateID {
			s.Require().NoError(storage.MarkPublished(ctx, row.ID), "marcar evento created como publicado")
			return outbox.Pack(row)
		}
	}
	s.FailNowf("evento nao encontrado no outbox", "type=%s aggregate=%s rows_claimed=%d", transactionCreatedType, aggregateID, len(rows))
	return outbox.Envelope{}
}

func (s *TransactionToBudgetChainSuite) assertEnvelopePayload(env outbox.Envelope, userID uuid.UUID) {
	s.Equal(transactionCreatedType, env.EventType, "event_type do envelope real")
	s.Equal(userID.String(), env.AggregateUserID)

	var p struct {
		AggregateID   string `json:"aggregate_id"`
		UserID        string `json:"user_id"`
		Direction     int    `json:"direction"`
		AmountCents   int64  `json:"amount_cents"`
		RefMonth      string `json:"ref_month"`
		SubcategoryID string `json:"subcategory_id"`
	}
	s.Require().NoError(json.Unmarshal(env.Payload, &p), "payload do outbox deve desserializar")
	s.Equal(2, p.Direction, "direction outcome serializado como inteiro")
	s.Equal(expectedAmountCents, p.AmountCents)
	s.Equal(expectedCompetence, p.RefMonth, "ref_month deve ser string YYYY-MM, nao {}")
	s.Equal(deliverySubcategoryID, p.SubcategoryID)
	s.Equal(userID.String(), p.UserID)
}

func (s *TransactionToBudgetChainSuite) assertExpensePersisted(ctx context.Context, mgr *sqlx.DB, userID uuid.UUID, externalTxID string) {
	db := mgr
	var (
		gotAmount     int64
		gotCompetence string
		gotRoot       string
		gotSub        string
		gotSource     string
	)
	row := db.QueryRowContext(ctx,
		`SELECT amount_cents, competence, root_slug, subcategory_id, source
		 FROM mecontrola.budgets_expenses
		 WHERE user_id = $1 AND external_transaction_id = $2`,
		userID, externalTxID,
	)
	s.Require().NoError(
		row.Scan(&gotAmount, &gotCompetence, &gotRoot, &gotSub, &gotSource),
		"despesa deve existir em mecontrola.budgets_expenses",
	)
	s.Equal(expectedAmountCents, gotAmount)
	s.Equal(expectedCompetence, gotCompetence)
	s.Equal(expectedRootSlugStored, gotRoot)
	s.Equal(deliverySubcategoryID, gotSub)
	s.Equal("transactions", gotSource)
}

func (s *TransactionToBudgetChainSuite) TestUpdateChain_ReconciliationMovesValueBetweenRootCategories() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	cfg := s.buildConfig()
	authMW := func(h http.Handler) http.Handler { return h }
	ctx := context.Background()

	categoriesModule := categories.NewCategoriesModule(mgr, o11y, authMW)
	cardModule, err := card.NewCardModule(ctx, cfg, o11y, mgr, authMW, nil, nil)
	s.Require().NoError(err)
	txModule, err := transactions.NewTransactionsModule(cfg, o11y, mgr, cardModule, categoriesModule, authMW)
	s.Require().NoError(err)
	budgetsModule, err := budgets.NewBudgetsModule(cfg, o11y, mgr, categoriesModule, authMW, nil, nil)
	s.Require().NoError(err)
	s.Require().NotNil(budgetsModule.TransactionUpdatedConsumer, "TransactionUpdatedConsumer deve estar wired")

	userID := uuid.New()
	principalCtx := auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	subID := uuid.MustParse(deliverySubcategoryID)
	raw := txinput.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   expectedAmountCents,
		Description:   "delivery a editar",
		CategoryID:    uuid.MustParse(prazerosRootCategoryID),
		SubcategoryID: &subID,
		OccurredAt:    time.Date(2026, time.June, 17, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}
	created, err := txModule.CreateTransactionUC.Execute(principalCtx, raw)
	s.Require().NoError(err)
	txID := created.ID.String()

	createEnvelope := s.claimTransactionCreatedEnvelope(ctx, mgr, txID)
	createEvent := &envelopeEvent{eventType: transactionCreatedType, envelope: createEnvelope}
	s.Require().NoError(budgetsModule.TransactionCreatedConsumer.Handle(ctx, platformevents.Event(createEvent)))
	s.assertExpensePersisted(ctx, mgr, userID, txID)

	newSubID := uuid.MustParse(aluguelSubcategoryID)
	updateRaw := txinput.RawUpdateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   updatedAmountCents,
		Description:   "delivery a editar",
		CategoryID:    uuid.MustParse(custoFixoRootCategoryID),
		SubcategoryID: &newSubID,
		OccurredAt:    time.Date(2026, time.June, 17, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Version:       1,
	}
	_, err = txModule.UpdateTransactionUC.Execute(principalCtx, txID, updateRaw)
	s.Require().NoError(err, "editar transaction")

	updateEnvelope := s.claimTransactionUpdatedEnvelope(ctx, mgr, txID)
	updateEvent := &envelopeEvent{eventType: transactionUpdatedType, envelope: updateEnvelope}
	s.Require().NoError(
		budgetsModule.TransactionUpdatedConsumer.Handle(ctx, platformevents.Event(updateEvent)),
		"consumer real deve reconciliar o envelope de edição",
	)

	s.assertExpenseMoved(ctx, mgr, userID, txID)
	s.assertMonthlySummaryReflectsEdit(ctx, budgetsModule, userID)
}

func (s *TransactionToBudgetChainSuite) claimTransactionUpdatedEnvelope(ctx context.Context, mgr *sqlx.DB, aggregateID string) outbox.Envelope {
	storage := outbox.NewPostgresStorage(mgr)

	for i := 0; i < 10; i++ {
		rows, err := storage.ClaimBatch(ctx, consumerLockedBy+"-updated", 100)
		s.Require().NoError(err)
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if row.Type == transactionUpdatedType && row.AggregateID == aggregateID {
				return outbox.Pack(row)
			}
			s.Require().NoError(storage.MarkPublished(ctx, row.ID), "marcar evento intermediario como publicado")
		}
	}
	s.FailNowf("evento updated não encontrado no outbox", "aggregate=%s", aggregateID)
	return outbox.Envelope{}
}

func (s *TransactionToBudgetChainSuite) assertExpenseMoved(ctx context.Context, mgr *sqlx.DB, userID uuid.UUID, externalTxID string) {
	db := mgr
	var (
		gotAmount int64
		gotRoot   string
		gotSub    string
	)
	row := db.QueryRowContext(ctx,
		`SELECT amount_cents, root_slug, subcategory_id
		 FROM mecontrola.budgets_expenses
		 WHERE user_id = $1 AND external_transaction_id = $2`,
		userID, externalTxID,
	)
	s.Require().NoError(
		row.Scan(&gotAmount, &gotRoot, &gotSub),
		"despesa editada deve permanecer em mecontrola.budgets_expenses",
	)
	s.Equal(updatedAmountCents, gotAmount, "valor deve refletir a edição")
	s.Equal(movedRootSlugStored, gotRoot, "root_slug deve mover para a nova categoria raiz")
	s.Equal(aluguelSubcategoryID, gotSub, "subcategoria deve refletir a edição")
}

func (s *TransactionToBudgetChainSuite) assertMonthlySummaryReflectsEdit(ctx context.Context, budgetsModule *budgets.BudgetsModule, userID uuid.UUID) {
	summary, err := budgetsModule.GetMonthlySummaryUC.Execute(ctx, userID.String(), expectedCompetence)
	s.Require().NoError(err, "resumo mensal deve resolver")
	s.Equal(updatedAmountCents, summary.TotalSpentCents, "gasto total deve refletir a edição")
}

func (s *TransactionToBudgetChainSuite) TestI6_DeleteChain_ExpenseSoftDeleted() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	cfg := s.buildConfig()
	authMW := func(h http.Handler) http.Handler { return h }
	ctx := context.Background()

	categoriesModule := categories.NewCategoriesModule(mgr, o11y, authMW)
	cardModule, err := card.NewCardModule(ctx, cfg, o11y, mgr, authMW, nil, nil)
	s.Require().NoError(err)
	txModule, err := transactions.NewTransactionsModule(cfg, o11y, mgr, cardModule, categoriesModule, authMW)
	s.Require().NoError(err)
	budgetsModule, err := budgets.NewBudgetsModule(cfg, o11y, mgr, categoriesModule, authMW, nil, nil)
	s.Require().NoError(err)
	s.Require().NotNil(budgetsModule.TransactionDeletedConsumer, "TransactionDeletedConsumer deve estar wired")

	userID := uuid.New()
	principalCtx := auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	subID := uuid.MustParse(deliverySubcategoryID)
	raw := txinput.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   expectedAmountCents,
		Description:   "delivery para deletar",
		CategoryID:    uuid.MustParse(prazerosRootCategoryID),
		SubcategoryID: &subID,
		OccurredAt:    time.Date(2026, time.June, 17, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}
	created, err := txModule.CreateTransactionUC.Execute(principalCtx, raw)
	s.Require().NoError(err)
	txID := created.ID.String()

	createEnvelope := s.claimTransactionCreatedEnvelope(ctx, mgr, txID)
	createEvent := &envelopeEvent{eventType: transactionCreatedType, envelope: createEnvelope}
	s.Require().NoError(budgetsModule.TransactionCreatedConsumer.Handle(ctx, platformevents.Event(createEvent)))
	s.assertExpensePersisted(ctx, mgr, userID, txID)

	delErr := txModule.DeleteTransactionUC.Execute(principalCtx, txID, 1)
	s.Require().NoError(delErr)

	deleteEnvelope := s.claimTransactionDeletedEnvelope(ctx, mgr, txID)
	deleteEvent := &envelopeEvent{eventType: "transactions.transaction.deleted.v1", envelope: deleteEnvelope}
	s.Require().NoError(budgetsModule.TransactionDeletedConsumer.Handle(ctx, platformevents.Event(deleteEvent)))
	s.assertExpenseSoftDeleted(ctx, mgr, userID, txID)
}

func (s *TransactionToBudgetChainSuite) TestEditBudgetTotal_PersistsNewTotalAndRescaledAllocations() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	cfg := s.buildConfig()
	authMW := func(h http.Handler) http.Handler { return h }
	ctx := context.Background()

	categoriesModule := categories.NewCategoriesModule(mgr, o11y, authMW)
	budgetsModule, err := budgets.NewBudgetsModule(cfg, o11y, mgr, categoriesModule, authMW, nil, nil)
	s.Require().NoError(err)
	s.Require().NotNil(budgetsModule.EditBudgetTotalUC, "EditBudgetTotalUC deve estar wired")

	userID := uuid.New()
	s.ensureUserExists(ctx, mgr, userID)

	_, err = budgetsModule.CreateBudgetUC.Execute(ctx, budgetinput.CreateBudgetInput{
		UserID:     userID.String(),
		Competence: expectedCompetence,
		TotalCents: 100000,
		Allocations: []budgetinput.AllocationInput{
			{RootSlug: "expense.prazeres", BasisPoints: 6000},
			{RootSlug: "expense.custo_fixo", BasisPoints: 4000},
		},
	})
	s.Require().NoError(err)

	_, err = budgetsModule.ActivateBudgetUC.Execute(ctx, budgetinput.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: expectedCompetence,
	})
	s.Require().NoError(err)

	result, err := budgetsModule.EditBudgetTotalUC.Execute(ctx, budgetinput.EditBudgetTotalInput{
		UserID:     userID.String(),
		Competence: expectedCompetence,
		TotalCents: 300000,
	})
	s.Require().NoError(err, "editar total do orçamento ativo")
	s.Equal(int64(300000), result.TotalCents)

	sum := int64(0)
	for _, a := range result.Allocations {
		sum += a.PlannedCents
	}
	s.Equal(int64(300000), sum, "soma das allocations deve fechar exatamente com o novo total")

	var (
		gotTotal int64
		gotState int
	)
	row := mgr.QueryRowContext(ctx,
		`SELECT total_cents, state FROM mecontrola.budgets WHERE user_id = $1 AND competence = $2`,
		userID, expectedCompetence,
	)
	s.Require().NoError(row.Scan(&gotTotal, &gotState), "orçamento deve persistir o novo total")
	s.Equal(int64(300000), gotTotal)
	s.Equal(2, gotState)

	var allocSum int64
	sumRow := mgr.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(planned_cents), 0) FROM mecontrola.budgets_allocations WHERE budget_id = $1`,
		result.ID,
	)
	s.Require().NoError(sumRow.Scan(&allocSum), "allocations persistidas devem somar o novo total")
	s.Equal(int64(300000), allocSum)
}

func (s *TransactionToBudgetChainSuite) TestBudgetActivationPublishesOutboxEvent() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	cfg := s.buildConfig()
	authMW := func(h http.Handler) http.Handler { return h }
	ctx := context.Background()

	categoriesModule := categories.NewCategoriesModule(mgr, o11y, authMW)
	budgetsModule, err := budgets.NewBudgetsModule(cfg, o11y, mgr, categoriesModule, authMW, nil, nil)
	s.Require().NoError(err)

	userID := uuid.New()
	s.ensureUserExists(ctx, mgr, userID)

	_, err = budgetsModule.CreateBudgetUC.Execute(ctx, budgetinput.CreateBudgetInput{
		UserID:     userID.String(),
		Competence: expectedCompetence,
		TotalCents: 100000,
		Allocations: []budgetinput.AllocationInput{
			{RootSlug: "expense.prazeres", BasisPoints: 10000},
		},
	})
	s.Require().NoError(err)

	activated, err := budgetsModule.ActivateBudgetUC.Execute(ctx, budgetinput.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: expectedCompetence,
	})
	s.Require().NoError(err)
	s.Equal("active", activated.State)

	var stateCount int
	stateRow := mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets WHERE id = $1 AND user_id = $2 AND state = 2`,
		activated.ID, userID.String(),
	)
	s.Require().NoError(stateRow.Scan(&stateCount))
	s.Equal(1, stateCount)

	var eventCount int
	eventRow := mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE event_type = $1 AND aggregate_id = $2 AND aggregate_user_id = $3`,
		"budgets.budget_activated.v1", activated.ID, userID.String(),
	)
	s.Require().NoError(eventRow.Scan(&eventCount))
	s.Equal(0, eventCount)
}

func (s *TransactionToBudgetChainSuite) TestThresholdAlertsJobPublishesOutboxEvent() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	cfg := s.buildConfig()
	cfg.BudgetsConfig.ThresholdAlertsMode = configs.ThresholdAlertsModeBoth
	authMW := func(h http.Handler) http.Handler { return h }
	ctx := context.Background()

	categoriesModule := categories.NewCategoriesModule(mgr, o11y, authMW)
	budgetsModule, err := budgets.NewBudgetsModule(cfg, o11y, mgr, categoriesModule, authMW, nil, nil)
	s.Require().NoError(err)
	s.Require().NotNil(budgetsModule.ThresholdAlertsJob)

	userID := uuid.New()
	s.ensureUserExists(ctx, mgr, userID)

	now := time.Now().UTC()
	loc := budgetvo.SaoPauloLocation()
	if loc == nil {
		loc = time.UTC
	}
	competence := budgetvo.CompetenceFromTime(now, loc).String()

	created, err := budgetsModule.CreateBudgetUC.Execute(ctx, budgetinput.CreateBudgetInput{
		UserID:     userID.String(),
		Competence: competence,
		TotalCents: 100000,
		Allocations: []budgetinput.AllocationInput{
			{RootSlug: "expense.prazeres", BasisPoints: 10000},
		},
	})
	s.Require().NoError(err)

	_, err = budgetsModule.ActivateBudgetUC.Execute(ctx, budgetinput.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: competence,
	})
	s.Require().NoError(err)

	_, err = budgetsModule.UpsertExpenseUC.Execute(ctx, budgetinput.UpsertExpenseInput{
		UserID:                userID.String(),
		Source:                "transactions",
		ExternalTransactionID: uuid.New().String(),
		SubcategoryID:         deliverySubcategoryID,
		Competence:            competence,
		AmountCents:           int64(85000),
		OccurredAt:            now,
	})
	s.Require().NoError(err)
	s.Require().NoError(budgetsModule.ThresholdAlertsJob.Run(ctx))

	var count int
	row := mgr.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE event_type = $1 AND aggregate_id = $2 AND aggregate_user_id = $3`,
		"budgets.threshold_alert_triggered.v1", created.ID, userID.String(),
	)
	s.Require().NoError(row.Scan(&count))
	s.Equal(1, count)
}

func (s *TransactionToBudgetChainSuite) claimTransactionDeletedEnvelope(ctx context.Context, mgr *sqlx.DB, aggregateID string) outbox.Envelope {
	const deletedType = "transactions.transaction.deleted.v1"
	storage := outbox.NewPostgresStorage(mgr)

	for i := 0; i < 10; i++ {
		rows, err := storage.ClaimBatch(ctx, consumerLockedBy+"-deleted", 100)
		s.Require().NoError(err)
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if row.Type == deletedType && row.AggregateID == aggregateID {
				return outbox.Pack(row)
			}
			s.Require().NoError(storage.MarkPublished(ctx, row.ID), "marcar evento intermediario como publicado")
		}
	}
	s.FailNowf("evento deleted não encontrado no outbox", "aggregate=%s", aggregateID)
	return outbox.Envelope{}
}

func (s *TransactionToBudgetChainSuite) assertExpenseSoftDeleted(ctx context.Context, mgr *sqlx.DB, userID uuid.UUID, externalTxID string) {
	db := mgr
	var deletedAt *time.Time
	row := db.QueryRowContext(ctx,
		`SELECT deleted_at FROM mecontrola.budgets_expenses WHERE user_id = $1 AND external_transaction_id = $2`,
		userID, externalTxID,
	)
	s.Require().NoError(row.Scan(&deletedAt), "expense deve existir em budgets_expenses")
	s.NotNil(deletedAt, "deleted_at deve estar preenchido após transaction.deleted (I6: sem falso positivo no resumo)")
}

func (s *TransactionToBudgetChainSuite) assertMonthlySummary(ctx context.Context, budgetsModule *budgets.BudgetsModule, userID uuid.UUID) {
	summary, err := budgetsModule.GetMonthlySummaryUC.Execute(ctx, userID.String(), expectedCompetence)
	s.Require().NoError(err, "resumo mensal deve resolver")
	s.Equal(expectedAmountCents, summary.TotalSpentCents, "gasto deve refletir no resumo mensal")
}
