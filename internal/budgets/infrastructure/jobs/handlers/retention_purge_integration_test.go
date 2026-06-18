//go:build integration

package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

func TestRetentionPurgeIntegration_PurgesOldExpenses(t *testing.T) {
	db, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	factory := repositories.NewRepositoryFactory(o11y)
	unitOfWork := uow.NewUnitOfWork(db)
	uc := usecases.NewPurgeRetention(factory, unitOfWork, 100, o11y)
	job := handlers.NewRetentionPurge(uc, configs.BudgetsConfig{RetentionPurgeBatchSize: 100})

	expenseID := uuid.New()
	userID := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.budgets_expenses
		        (id, user_id, source, external_transaction_id, subcategory_id,
		         root_slug, competence, amount_cents, occurred_at,
		         version, tombstone_version, deleted_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		expenseID, userID, "api", uuid.New().String(), uuid.New(),
		"expense.prazeres", "2023-01", 10000, time.Now().UTC(),
		1, 0, time.Now().UTC().Add(-731*24*time.Hour), time.Now().UTC().Add(-731*24*time.Hour), time.Now().UTC().Add(-731*24*time.Hour),
	)
	require.NoError(t, err)

	require.NoError(t, job.Run(ctx))

	var count int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_expenses WHERE id = $1`,
		expenseID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestRetentionPurgeIntegration_PreservesRecentExpenses(t *testing.T) {
	db, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	factory := repositories.NewRepositoryFactory(o11y)
	unitOfWork := uow.NewUnitOfWork(db)
	uc := usecases.NewPurgeRetention(factory, unitOfWork, 100, o11y)
	job := handlers.NewRetentionPurge(uc, configs.BudgetsConfig{RetentionPurgeBatchSize: 100})

	expenseID := uuid.New()
	userID := uuid.New()
	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.budgets_expenses
		        (id, user_id, source, external_transaction_id, subcategory_id,
		         root_slug, competence, amount_cents, occurred_at,
		         version, tombstone_version, deleted_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		expenseID, userID, "api", uuid.New().String(), uuid.New(),
		"expense.prazeres", "2026-06", 10000, yesterday,
		1, 0, yesterday, yesterday, yesterday,
	)
	require.NoError(t, err)

	require.NoError(t, job.Run(ctx))

	var count int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_expenses WHERE id = $1`,
		expenseID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestRetentionPurgeIntegration_DoubleRunIsIdempotent(t *testing.T) {
	db, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	factory := repositories.NewRepositoryFactory(o11y)
	unitOfWork := uow.NewUnitOfWork(db)
	uc := usecases.NewPurgeRetention(factory, unitOfWork, 100, o11y)
	job := handlers.NewRetentionPurge(uc, configs.BudgetsConfig{RetentionPurgeBatchSize: 100})

	expenseID := uuid.New()
	userID := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.budgets_expenses
		        (id, user_id, source, external_transaction_id, subcategory_id,
		         root_slug, competence, amount_cents, occurred_at,
		         version, tombstone_version, deleted_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		expenseID, userID, "api", uuid.New().String(), uuid.New(),
		"expense.prazeres", "2023-01", 10000, time.Now().UTC(),
		1, 0, time.Now().UTC().Add(-731*24*time.Hour), time.Now().UTC().Add(-731*24*time.Hour), time.Now().UTC().Add(-731*24*time.Hour),
	)
	require.NoError(t, err)

	require.NoError(t, job.Run(ctx))
	require.NoError(t, job.Run(ctx))
}
