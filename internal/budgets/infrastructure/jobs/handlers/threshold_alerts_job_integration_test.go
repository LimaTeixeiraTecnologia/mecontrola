//go:build integration

package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

func mustThresholdRatio(t *testing.T, v float64) valueobjects.ThresholdRatio {
	t.Helper()
	r, err := valueobjects.NewThresholdRatio(v)
	require.NoError(t, err)
	return r
}

func insertActiveBudgetWithAllocation(t *testing.T, db *sqlx.DB, userID, budgetID uuid.UUID, competence string, totalCents int64, rootSlug string, basisPoints int, plannedCents int64) {
	t.Helper()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.budgets (id, user_id, competence, total_cents, state, activated_at, auto_draft, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 2, NOW(), false, NOW(), NOW())`,
		budgetID, userID, competence, totalCents,
	)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO mecontrola.budgets_allocations (budget_id, root_slug, basis_points, planned_cents)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (budget_id, root_slug) DO UPDATE
		    SET basis_points = EXCLUDED.basis_points, planned_cents = EXCLUDED.planned_cents`,
		budgetID, rootSlug, basisPoints, plannedCents,
	)
	require.NoError(t, err)
}

func insertExpenseForBudget(t *testing.T, db *sqlx.DB, userID uuid.UUID, competence string, rootSlug string, amountCents int64) {
	t.Helper()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.budgets_expenses
		        (id, user_id, source, external_transaction_id, subcategory_id,
		         root_slug, competence, amount_cents, occurred_at,
		         version, tombstone_version, deleted_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		uuid.New(), userID, "api", uuid.New().String(), uuid.New(),
		rootSlug, competence, amountCents, time.Now().UTC(),
		1, 0, nil, time.Now().UTC(), time.Now().UTC(),
	)
	require.NoError(t, err)
}

func TestThresholdAlertsJobIntegration_DispatchesAlertWhenAboveThreshold(t *testing.T) {
	db, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	factory := repositories.NewRepositoryFactory(o11y)
	unitOfWork := uow.NewUnitOfWork(db)

	publisher := mocks.NewThresholdAlertPublisher(t)
	publisher.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	thresholdConfig := services.ThresholdConfig{
		Category: mustThresholdRatio(t, 0.80),
		Goal:     mustThresholdRatio(t, 0.50),
	}

	uc := usecases.NewEvaluateThresholdAlerts(factory, publisher, unitOfWork, thresholdConfig, time.UTC, 500, o11y)
	job := handlers.NewThresholdAlertsJob(uc, configs.BudgetsConfig{ThresholdAlertsCron: "@hourly"})

	now := time.Now().UTC()
	competence := now.Format("2006-01")

	userID := uuid.New()
	budgetID := uuid.New()

	insertActiveBudgetWithAllocation(t, db, userID, budgetID, competence, 100000, "expense.prazeres", 10000, 100000)
	insertExpenseForBudget(t, db, userID, competence, "expense.prazeres", 85000)

	require.NoError(t, job.Run(ctx))

	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budget_alerts_sent WHERE budget_id = $1`,
		budgetID,
	).Scan(&count)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, 1)
}

func TestThresholdAlertsJobIntegration_DoubleRunDoesNotDuplicateAlerts(t *testing.T) {
	db, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	factory := repositories.NewRepositoryFactory(o11y)
	unitOfWork := uow.NewUnitOfWork(db)

	publisher := mocks.NewThresholdAlertPublisher(t)
	publisher.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	thresholdConfig := services.ThresholdConfig{
		Category: mustThresholdRatio(t, 0.80),
		Goal:     mustThresholdRatio(t, 0.50),
	}

	uc := usecases.NewEvaluateThresholdAlerts(factory, publisher, unitOfWork, thresholdConfig, time.UTC, 500, o11y)
	job := handlers.NewThresholdAlertsJob(uc, configs.BudgetsConfig{ThresholdAlertsCron: "@hourly"})

	now := time.Now().UTC()
	competence := now.Format("2006-01")

	userID := uuid.New()
	budgetID := uuid.New()

	insertActiveBudgetWithAllocation(t, db, userID, budgetID, competence, 100000, "expense.prazeres", 10000, 100000)
	insertExpenseForBudget(t, db, userID, competence, "expense.prazeres", 85000)

	require.NoError(t, job.Run(ctx))

	var countAfterFirst int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budget_alerts_sent WHERE budget_id = $1`,
		budgetID,
	).Scan(&countAfterFirst)
	require.NoError(t, err)

	uc2 := usecases.NewEvaluateThresholdAlerts(factory, publisher, uow.NewUnitOfWork(db), thresholdConfig, time.UTC, 500, o11y)
	job2 := handlers.NewThresholdAlertsJob(uc2, configs.BudgetsConfig{ThresholdAlertsCron: "@hourly"})

	require.NoError(t, job2.Run(ctx))

	var countAfterSecond int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budget_alerts_sent WHERE budget_id = $1`,
		budgetID,
	).Scan(&countAfterSecond)
	require.NoError(t, err)
	require.Equal(t, countAfterFirst, countAfterSecond)
}
