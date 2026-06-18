//go:build integration

package consumers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

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

type ExpenseCommittedConsumerIntegrationSuite struct {
	suite.Suite
}

func TestExpenseCommittedConsumerIntegration(t *testing.T) {
	suite.Run(t, new(ExpenseCommittedConsumerIntegrationSuite))
}

func (s *ExpenseCommittedConsumerIntegrationSuite) newModule(db *sqlx.DB) *budgets.BudgetsModule {
	o11y := noop.NewProvider()
	cfg, err := configs.LoadConfig("../../../../../../")
	s.Require().NoError(err)
	authMW := func(h http.Handler) http.Handler { return h }
	catModule := categories.NewCategoriesModule(db, o11y, authMW)
	mod, err := budgets.NewBudgetsModule(cfg, o11y, db, catModule, authMW, nil, nil)
	s.Require().NoError(err)
	return mod
}

func (s *ExpenseCommittedConsumerIntegrationSuite) seedActiveBudget(ctx context.Context, mod *budgets.BudgetsModule, userID uuid.UUID) {
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

func (s *ExpenseCommittedConsumerIntegrationSuite) buildEnvelope(eventID, userID string) outbox.Envelope {
	raw, _ := json.Marshal(map[string]any{
		"user_id":              userID,
		"competence":           "2026-06",
		"subcategory_id":       uuid.New().String(),
		"root_slug":            "expense.prazeres",
		"mutation_kind":        "create",
		"committed_at":         "2026-06-17T12:00:00Z",
		"cutoff_competence_br": "2026-06",
	})
	return outbox.Envelope{ID: eventID, Payload: raw}
}

func (s *ExpenseCommittedConsumerIntegrationSuite) TestEvaluatesAlertWithActiveBudget() {
	db, _ := testcontainer.Postgres(s.T())
	mod := s.newModule(db)
	ctx := context.Background()

	userID := uuid.New()
	eventID := uuid.New().String()

	s.seedActiveBudget(ctx, mod, userID)

	env := s.buildEnvelope(eventID, userID.String())
	evt := stubEvent{eventType: "budgets.expense.committed.v1", payload: env}
	err := mod.ExpenseCommittedConsumer.Handle(ctx, platformevents.Event(evt))
	s.Require().NoError(err)
}

func (s *ExpenseCommittedConsumerIntegrationSuite) TestInvalidUserIDReturnsError() {
	db, _ := testcontainer.Postgres(s.T())
	mod := s.newModule(db)
	ctx := context.Background()

	raw, _ := json.Marshal(map[string]any{
		"user_id":              "not-a-valid-uuid",
		"competence":           "2026-06",
		"subcategory_id":       uuid.New().String(),
		"root_slug":            "expense.prazeres",
		"mutation_kind":        "create",
		"committed_at":         "2026-06-17T12:00:00Z",
		"cutoff_competence_br": "2026-06",
	})
	env := outbox.Envelope{ID: uuid.New().String(), Payload: raw}
	evt := stubEvent{eventType: "budgets.expense.committed.v1", payload: env}
	err := mod.ExpenseCommittedConsumer.Handle(ctx, platformevents.Event(evt))
	s.Require().Error(err)

	var count int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_alerts`,
	).Scan(&count))
	s.Equal(0, count)
}
