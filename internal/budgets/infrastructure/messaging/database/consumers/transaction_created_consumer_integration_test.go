//go:build integration

package consumers_test

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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type TransactionCreatedConsumerIntegrationSuite struct {
	suite.Suite
}

func TestTransactionCreatedConsumerIntegration(t *testing.T) {
	suite.Run(t, new(TransactionCreatedConsumerIntegrationSuite))
}

func (s *TransactionCreatedConsumerIntegrationSuite) newModule(db *sqlx.DB) *budgets.BudgetsModule {
	o11y := noop.NewProvider()
	cfg, err := configs.LoadConfig("../../../../../../")
	s.Require().NoError(err)
	authMW := func(h http.Handler) http.Handler { return h }
	catModule := categories.NewCategoriesModule(db, o11y, authMW)
	mod, err := budgets.NewBudgetsModule(cfg, o11y, db, catModule, authMW, nil, nil)
	s.Require().NoError(err)
	return mod
}

func (s *TransactionCreatedConsumerIntegrationSuite) seedActiveBudget(ctx context.Context, mod *budgets.BudgetsModule, userID uuid.UUID) {
	_, err := mod.CreateBudgetUC.Execute(ctx, budgetinput.CreateBudgetInput{
		UserID:     userID.String(),
		Competence: "2026-06",
		TotalCents: 100000,
		Allocations: []budgetinput.AllocationInput{
			{RootSlug: "expense.prazeres", BasisPoints: 10000},
		},
	})
	s.Require().NoError(err)
	_, err = mod.ActivateBudgetUC.Execute(ctx, budgetinput.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: "2026-06",
	})
	s.Require().NoError(err)
}

func (s *TransactionCreatedConsumerIntegrationSuite) buildOutcomeEnvelope(aggregateID, userID string) outbox.Envelope {
	raw, _ := json.Marshal(map[string]any{
		"aggregate_id":   aggregateID,
		"user_id":        userID,
		"occurred_at":    time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
		"direction":      2,
		"amount_cents":   int64(5800),
		"ref_month":      "2026-06",
		"subcategory_id": "ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c",
	})
	return outbox.Envelope{ID: uuid.New().String(), Payload: raw}
}

func (s *TransactionCreatedConsumerIntegrationSuite) TestCreatesExpense() {
	db, _ := testcontainer.Postgres(s.T())
	mod := s.newModule(db)
	ctx := context.Background()

	userID := uuid.New()
	aggregateID := uuid.New().String()

	s.seedActiveBudget(ctx, mod, userID)

	env := s.buildOutcomeEnvelope(aggregateID, userID.String())
	evt := stubEvent{eventType: "transactions.transaction.created.v1", payload: env}
	s.Require().NoError(mod.TransactionCreatedConsumer.Handle(ctx, platformevents.Event(evt)))

	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_expenses WHERE user_id = $1 AND external_transaction_id = $2`,
		userID.String(), aggregateID,
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)
}

func (s *TransactionCreatedConsumerIntegrationSuite) TestIdempotency() {
	db, _ := testcontainer.Postgres(s.T())
	mod := s.newModule(db)
	ctx := context.Background()

	userID := uuid.New()
	aggregateID := uuid.New().String()

	s.seedActiveBudget(ctx, mod, userID)

	env := s.buildOutcomeEnvelope(aggregateID, userID.String())
	evt := stubEvent{eventType: "transactions.transaction.created.v1", payload: env}

	s.Require().NoError(mod.TransactionCreatedConsumer.Handle(ctx, platformevents.Event(evt)))
	s.Require().NoError(mod.TransactionCreatedConsumer.Handle(ctx, platformevents.Event(evt)))

	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_expenses WHERE user_id = $1 AND external_transaction_id = $2`,
		userID.String(), aggregateID,
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)
}
