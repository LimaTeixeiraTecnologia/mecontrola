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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type ExternalExpenseConsumerIntegrationSuite struct {
	suite.Suite
}

func TestExternalExpenseConsumerIntegration(t *testing.T) {
	suite.Run(t, new(ExternalExpenseConsumerIntegrationSuite))
}

func (s *ExternalExpenseConsumerIntegrationSuite) newModule(db *sqlx.DB) *budgets.BudgetsModule {
	o11y := noop.NewProvider()
	cfg, err := configs.LoadConfig("../../../../../../")
	s.Require().NoError(err)
	authMW := func(h http.Handler) http.Handler { return h }
	catModule := categories.NewCategoriesModule(db, o11y, authMW)
	mod, err := budgets.NewBudgetsModule(cfg, o11y, db, catModule, authMW, nil, nil)
	s.Require().NoError(err)
	return mod
}

func (s *ExternalExpenseConsumerIntegrationSuite) buildEnvelope(eventID, userID string) outbox.Envelope {
	raw, _ := json.Marshal(map[string]any{
		"event_id":                eventID,
		"source":                  "kiwify",
		"external_transaction_id": uuid.New().String(),
		"occurred_at":             time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
		"user_id":                 userID,
		"operation":               "create",
		"version":                 int64(1),
		"subcategory_id":          "ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c",
		"competence":              "2026-06",
		"amount_cents":            int64(5000),
	})
	return outbox.Envelope{ID: uuid.New().String(), Payload: raw}
}

func (s *ExternalExpenseConsumerIntegrationSuite) TestInsertsIntoPendingOrExpense() {
	db, _ := testcontainer.Postgres(s.T())
	mod := s.newModule(db)
	ctx := context.Background()

	userID := uuid.New()
	eventID := uuid.New().String()

	env := s.buildEnvelope(eventID, userID.String())
	evt := stubEvent{eventType: "external.expense.v1", payload: env}
	s.Require().NoError(mod.ExternalExpenseConsumer.Handle(ctx, platformevents.Event(evt)))

	var expenseCount int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_expenses WHERE user_id = $1`,
		userID.String(),
	).Scan(&expenseCount)
	s.Require().NoError(err)

	var pendingCount int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_expense_events_pending WHERE user_id = $1 AND event_id = $2`,
		userID.String(), eventID,
	).Scan(&pendingCount)
	s.Require().NoError(err)

	s.True(expenseCount+pendingCount == 1, "evento deve ter sido processado: expense ou pending deve ter exatamente 1 linha")
}

func (s *ExternalExpenseConsumerIntegrationSuite) TestIdempotency() {
	db, _ := testcontainer.Postgres(s.T())
	mod := s.newModule(db)
	ctx := context.Background()

	userID := uuid.New()
	eventID := uuid.New().String()

	env := s.buildEnvelope(eventID, userID.String())
	evt := stubEvent{eventType: "external.expense.v1", payload: env}

	s.Require().NoError(mod.ExternalExpenseConsumer.Handle(ctx, platformevents.Event(evt)))
	s.Require().NoError(mod.ExternalExpenseConsumer.Handle(ctx, platformevents.Event(evt)))

	var expenseCount int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_expenses WHERE user_id = $1`,
		userID.String(),
	).Scan(&expenseCount))

	var pendingCount int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_expense_events_pending WHERE user_id = $1 AND event_id = $2`,
		userID.String(), eventID,
	).Scan(&pendingCount))

	s.True(expenseCount+pendingCount == 1, "reprocessamento nao deve duplicar: total deve permanecer 1")
}
