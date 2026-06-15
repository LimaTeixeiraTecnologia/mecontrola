//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	budgetpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

func setupTestDB(t *testing.T) manager.Manager {
	t.Helper()
	mgr, _ := testcontainer.Postgres(t)
	return mgr
}

func testO11y() observability.Observability {
	return noop.NewProvider()
}

func newBudgetRepo(o11y observability.Observability, db database.DBTX) interfaces.BudgetRepository {
	return budgetpostgres.NewBudgetRepository(o11y, db)
}

func newExpenseRepo(o11y observability.Observability, db database.DBTX) interfaces.ExpenseRepository {
	return budgetpostgres.NewExpenseRepository(o11y, db)
}

func newAlertRepo(o11y observability.Observability, db database.DBTX) interfaces.AlertRepository {
	return budgetpostgres.NewAlertRepository(o11y, db)
}

func newThresholdStateRepo(o11y observability.Observability, db database.DBTX) interfaces.ThresholdStateRepository {
	return budgetpostgres.NewThresholdStateRepository(o11y, db)
}

func newPendingEventRepo(o11y observability.Observability, db database.DBTX) interfaces.PendingEventRepository {
	return budgetpostgres.NewPendingEventRepository(o11y, db)
}

func mustCompetence(t *testing.T, s string) valueobjects.Competence {
	t.Helper()
	c, err := valueobjects.NewCompetence(s)
	require.NoError(t, err)
	return c
}

func mustRootSlug(t *testing.T, s string) valueobjects.RootSlug {
	t.Helper()
	r, err := valueobjects.ParseRootSlug(s)
	require.NoError(t, err)
	return r
}

func mustProducerSource(t *testing.T, s string) valueobjects.ProducerSource {
	t.Helper()
	ps, err := valueobjects.NewProducerSource(s)
	require.NoError(t, err)
	return ps
}

func mustExternalTransactionID(t *testing.T, s string) valueobjects.ExternalTransactionID {
	t.Helper()
	e, err := valueobjects.NewExternalTransactionID(s)
	require.NoError(t, err)
	return e
}

func mustThreshold(t *testing.T, v int) valueobjects.Threshold {
	t.Helper()
	th, err := valueobjects.ParseThreshold(v)
	require.NoError(t, err)
	return th
}

func newTestBudget(userID uuid.UUID, competence valueobjects.Competence) entities.Budget {
	return entities.NewBudget(userID, competence, 500000, time.Now().UTC())
}

func newTestExpense(t *testing.T, userID uuid.UUID, source, extID, competence, rootSlug string) entities.Expense {
	t.Helper()
	subcategoryID := uuid.New()
	e, err := entities.NewExpense(
		userID,
		mustProducerSource(t, source),
		mustExternalTransactionID(t, extID),
		subcategoryID,
		mustRootSlug(t, rootSlug),
		mustCompetence(t, competence),
		10000,
		time.Now().UTC(),
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("newTestExpense: %v", err)
	}
	return e
}
