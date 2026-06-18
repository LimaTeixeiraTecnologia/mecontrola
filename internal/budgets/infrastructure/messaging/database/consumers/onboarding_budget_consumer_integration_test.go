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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type OnboardingBudgetConsumerIntegrationSuite struct {
	suite.Suite
}

func TestOnboardingBudgetConsumerIntegration(t *testing.T) {
	suite.Run(t, new(OnboardingBudgetConsumerIntegrationSuite))
}

func (s *OnboardingBudgetConsumerIntegrationSuite) newModule(db *sqlx.DB) *budgets.BudgetsModule {
	o11y := noop.NewProvider()
	cfg, err := configs.LoadConfig("../../../../../../")
	s.Require().NoError(err)
	authMW := func(h http.Handler) http.Handler { return h }
	catModule := categories.NewCategoriesModule(db, o11y, authMW)
	mod, err := budgets.NewBudgetsModule(cfg, o11y, db, catModule, authMW, nil, nil)
	s.Require().NoError(err)
	return mod
}

func (s *OnboardingBudgetConsumerIntegrationSuite) buildEnvelope(userID string) outbox.Envelope {
	raw, _ := json.Marshal(map[string]any{
		"UserID":      userID,
		"IncomeCents": int64(100000),
		"Allocations": []map[string]any{
			{"Kind": "fixed_cost", "Percent": 55},
			{"Kind": "pleasures", "Percent": 10},
			{"Kind": "knowledge", "Percent": 10},
			{"Kind": "goals", "Percent": 15},
			{"Kind": "financial_freedom", "Percent": 10},
		},
	})
	return outbox.Envelope{ID: uuid.New().String(), Payload: raw}
}

func (s *OnboardingBudgetConsumerIntegrationSuite) TestCreatesBudget() {
	db, _ := testcontainer.Postgres(s.T())
	mod := s.newModule(db)
	ctx := context.Background()

	userID := uuid.New()

	env := s.buildEnvelope(userID.String())
	evt := stubEvent{eventType: "onboarding.splits_calculated", payload: env}
	s.Require().NoError(mod.OnboardingBudgetConsumer.Handle(ctx, platformevents.Event(evt)))

	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets WHERE user_id = $1`,
		userID.String(),
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)
}

func (s *OnboardingBudgetConsumerIntegrationSuite) TestIdempotency() {
	db, _ := testcontainer.Postgres(s.T())
	mod := s.newModule(db)
	ctx := context.Background()

	userID := uuid.New()

	env := s.buildEnvelope(userID.String())
	evt := stubEvent{eventType: "onboarding.splits_calculated", payload: env}

	s.Require().NoError(mod.OnboardingBudgetConsumer.Handle(ctx, platformevents.Event(evt)))
	s.Require().NoError(mod.OnboardingBudgetConsumer.Handle(ctx, platformevents.Event(evt)))

	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets WHERE user_id = $1`,
		userID.String(),
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)
}
