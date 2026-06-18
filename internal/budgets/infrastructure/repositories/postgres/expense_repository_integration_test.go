//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExpenseRepositorySuite struct {
	suite.Suite
}

func TestExpenseRepositorySuite(t *testing.T) {
	suite.Run(t, new(ExpenseRepositorySuite))
}

func (s *ExpenseRepositorySuite) TestInsertAndGetByIdentity() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newExpenseRepo(testO11y(), mgr)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, expense))

	identity := expense.Identity()
	found, tombstone, err := repo.GetByIdentity(ctx, identity)
	s.Require().NoError(err)
	s.Assert().Equal(expense.ID(), found.ID())
	s.Assert().Equal(int64(1), found.Version())
	s.Assert().False(tombstone.IsPresent())
}

func (s *ExpenseRepositorySuite) TestGetByIdentityNotFound() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newExpenseRepo(testO11y(), mgr)

	userID := uuid.New()
	src, _ := valueobjects.NewProducerSource("api")
	extID, _ := valueobjects.NewExternalTransactionID(newUUIDv4())
	identity := entities.ExpenseIdentity{
		UserID:                userID,
		Source:                src,
		ExternalTransactionID: extID,
	}

	_, _, err := repo.GetByIdentity(ctx, identity)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrExpenseNotFound))
}

func (s *ExpenseRepositorySuite) TestInsertDuplicateConflict() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newExpenseRepo(testO11y(), mgr)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, expense))

	expense2 := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")
	err := repo.Insert(ctx, expense2)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrExpenseConflict))
}

func (s *ExpenseRepositorySuite) TestUpdate() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newExpenseRepo(testO11y(), mgr)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, expense))

	subcategoryID := uuid.New()
	rootSlug := mustRootSlug(s.T(), "expense.conhecimento")
	competence := mustCompetence(s.T(), "2025-01")
	now := time.Now().UTC()
	s.Require().NoError(expense.Edit(subcategoryID, rootSlug, competence, 20000, now, 1, now))

	s.Require().NoError(repo.Update(ctx, expense, 1))

	identity := expense.Identity()
	found, _, err := repo.GetByIdentity(ctx, identity)
	s.Require().NoError(err)
	s.Assert().Equal(int64(2), found.Version())
	s.Assert().Equal(int64(20000), found.AmountCents())
}

func (s *ExpenseRepositorySuite) TestUpdateVersionConflict() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newExpenseRepo(testO11y(), mgr)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, expense))

	subcategoryID := uuid.New()
	rootSlug := mustRootSlug(s.T(), "expense.conhecimento")
	competence := mustCompetence(s.T(), "2025-01")
	now := time.Now().UTC()
	s.Require().NoError(expense.Edit(subcategoryID, rootSlug, competence, 20000, now, 1, now))

	err := repo.Update(ctx, expense, 99)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrExpenseConflict))
}

func (s *ExpenseRepositorySuite) TestSoftDelete() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newExpenseRepo(testO11y(), mgr)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, expense))

	now := time.Now().UTC()
	tombstoneVersion, softDeleteErr := expense.SoftDelete(1, now)
	s.Require().NoError(softDeleteErr)

	persistedTombstoneVersion, err := repo.SoftDelete(ctx, expense, 1)
	s.Require().NoError(err)
	s.Assert().Equal(tombstoneVersion, persistedTombstoneVersion)

	identity := expense.Identity()
	found, tombstone, err := repo.GetByIdentity(ctx, identity)
	s.Require().NoError(err)
	s.Assert().True(found.IsDeleted())
	s.Assert().True(tombstone.IsPresent())
	s.Assert().Equal(tombstoneVersion, tombstone.TombstoneVersion())
}

func (s *ExpenseRepositorySuite) TestSoftDeleteVersionConflict() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newExpenseRepo(testO11y(), mgr)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, expense))

	now := time.Now().UTC()
	_, softDeleteErr := expense.SoftDelete(1, now)
	s.Require().NoError(softDeleteErr)

	_, err := repo.SoftDelete(ctx, expense, 99)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrExpenseConflict))
}

func (s *ExpenseRepositorySuite) TestTombstoneBlocksReuse() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newExpenseRepo(testO11y(), mgr)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, expense))

	now := time.Now().UTC()
	_, softDeleteErr := expense.SoftDelete(1, now)
	s.Require().NoError(softDeleteErr)
	_, err := repo.SoftDelete(ctx, expense, 1)
	s.Require().NoError(err)

	expense2 := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")
	err = repo.Insert(ctx, expense2)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrExpenseConflict))
}

func (s *ExpenseRepositorySuite) TestSumByRoot() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	repo := newExpenseRepo(testO11y(), mgr)

	userID := uuid.New()
	competence := mustCompetence(s.T(), "2025-02")

	for i := 0; i < 3; i++ {
		extID := newUUIDv4()
		e, expErr := entities.NewExpense(
			userID,
			mustProducerSource(s.T(), "api"),
			mustExternalTransactionID(s.T(), extID),
			uuid.New(),
			mustRootSlug(s.T(), "expense.custo_fixo"),
			competence,
			5000,
			time.Now().UTC(),
			time.Now().UTC(),
		)
		s.Require().NoError(expErr)
		s.Require().NoError(repo.Insert(ctx, e))
	}

	deletedExtID := newUUIDv4()
	deletedExpense, delExpErr := entities.NewExpense(
		userID,
		mustProducerSource(s.T(), "api"),
		mustExternalTransactionID(s.T(), deletedExtID),
		uuid.New(),
		mustRootSlug(s.T(), "expense.custo_fixo"),
		competence,
		9999,
		time.Now().UTC(),
		time.Now().UTC(),
	)
	s.Require().NoError(delExpErr)
	s.Require().NoError(repo.Insert(ctx, deletedExpense))
	_, softDeleteErr := deletedExpense.SoftDelete(1, time.Now().UTC())
	s.Require().NoError(softDeleteErr)
	_, err := repo.SoftDelete(ctx, deletedExpense, 1)
	s.Require().NoError(err)

	sums, err := repo.SumByRoot(ctx, userID, competence)
	s.Require().NoError(err)

	custoFixo := mustRootSlug(s.T(), "expense.custo_fixo")
	s.Assert().Equal(int64(15000), sums[custoFixo])
	_, hasDeleted := sums[mustRootSlug(s.T(), "expense.conhecimento")]
	s.Assert().False(hasDeleted)
}

func (s *ExpenseRepositorySuite) TestSumByRootExplainUsesIndex() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	db := mgr

	userID := uuid.New()
	competence := mustCompetence(s.T(), "2025-03")

	for i := range 64 {
		expense, err := entities.NewExpense(
			userID,
			mustProducerSource(s.T(), "api"),
			mustExternalTransactionID(s.T(), newUUIDv4()),
			uuid.New(),
			mustRootSlug(s.T(), "expense.custo_fixo"),
			competence,
			int64(1000+i),
			time.Now().UTC(),
			time.Now().UTC(),
		)
		s.Require().NoError(err)
		s.Require().NoError(newExpenseRepo(testO11y(), db).Insert(ctx, expense))
	}

	_, err := db.ExecContext(ctx, `ANALYZE mecontrola.budgets_expenses`)
	s.Require().NoError(err)

	explainQuery := `EXPLAIN (FORMAT TEXT) SELECT root_slug, SUM(amount_cents) FROM mecontrola.budgets_expenses WHERE user_id = $1 AND competence = $2 AND deleted_at IS NULL GROUP BY root_slug`

	tx, err := mgr.BeginTx(ctx, nil)
	s.Require().NoError(err)
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `SET LOCAL enable_seqscan = off`)
	s.Require().NoError(err)

	rows, err := tx.QueryContext(ctx, explainQuery, userID, competence.String())
	s.Require().NoError(err)
	defer func() { _ = rows.Close() }()

	var plan strings.Builder
	for rows.Next() {
		var line string
		s.Require().NoError(rows.Scan(&line))
		plan.WriteString(line)
		plan.WriteString("\n")
	}
	s.Require().NoError(rows.Err())

	planText := plan.String()
	s.Assert().True(
		strings.Contains(planText, "Index") || strings.Contains(planText, "Bitmap"),
		fmt.Sprintf("expected index scan in plan, got:\n%s", planText),
	)
}

func newUUIDv4() string {
	return uuid.New().String()
}
