//go:build integration

package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

func insertAbandonedDraftBudget(t *testing.T, db *sqlx.DB, userID uuid.UUID, competence string, daysAgo int) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO mecontrola.budgets (id, user_id, competence, total_cents, state, auto_draft, created_at, updated_at)
		 VALUES ($1, $2, $3, 100000, 1, false, NOW() - ($4 * INTERVAL '1 day'), NOW() - ($4 * INTERVAL '1 day'))`,
		id, userID, competence, daysAgo,
	)
	require.NoError(t, err)
	return id
}

func TestAbandonedDraftReaperIntegration_SignalsOldDrafts(t *testing.T) {
	db, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	factory := repositories.NewRepositoryFactory(o11y)
	unitOfWork := uow.NewUnitOfWork(db)
	uc := usecases.NewSignalAbandonedDrafts(factory, unitOfWork, time.UTC, o11y)
	job := handlers.NewAbandonedDraftReaper(uc, configs.BudgetsConfig{AbandonedDraftCron: "@daily"})

	userID := uuid.New()
	insertAbandonedDraftBudget(t, db, userID, "2025-01", 31)
	insertAbandonedDraftBudget(t, db, userID, "2025-02", 32)

	require.NoError(t, job.Run(ctx))

	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_abandoned_draft_signals`,
	).Scan(&count)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, 2)
}

func TestAbandonedDraftReaperIntegration_DoubleRunIsIdempotent(t *testing.T) {
	db, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	factory := repositories.NewRepositoryFactory(o11y)
	unitOfWork := uow.NewUnitOfWork(db)
	uc := usecases.NewSignalAbandonedDrafts(factory, unitOfWork, time.UTC, o11y)
	job := handlers.NewAbandonedDraftReaper(uc, configs.BudgetsConfig{AbandonedDraftCron: "@daily"})

	userID := uuid.New()
	insertAbandonedDraftBudget(t, db, userID, "2025-03", 31)

	require.NoError(t, job.Run(ctx))

	var countAfterFirst int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_abandoned_draft_signals`,
	).Scan(&countAfterFirst)
	require.NoError(t, err)

	require.NoError(t, job.Run(ctx))

	var countAfterSecond int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.budgets_abandoned_draft_signals`,
	).Scan(&countAfterSecond)
	require.NoError(t, err)
	require.Equal(t, countAfterFirst, countAfterSecond)
}
