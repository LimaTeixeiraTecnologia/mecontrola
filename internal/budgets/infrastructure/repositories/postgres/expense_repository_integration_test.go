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
	repo := newExpenseRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, db, expense))

	identity := expense.Identity()
	found, tombstone, err := repo.GetByIdentity(ctx, db, identity)
	s.Require().NoError(err)
	s.Assert().Equal(expense.ID(), found.ID())
	s.Assert().Equal(int64(1), found.Version())
	s.Assert().False(tombstone.IsPresent())
}

func (s *ExpenseRepositorySuite) TestGetByIdentityNotFound() {
	mgr := setupTestDB(s.T())
	repo := newExpenseRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	userID := uuid.New()
	src, _ := valueobjects.NewProducerSource("api")
	extID, _ := valueobjects.NewExternalTransactionID(newUUIDv4())
	identity := entities.ExpenseIdentity{
		UserID:                userID,
		Source:                src,
		ExternalTransactionID: extID,
	}

	_, _, err := repo.GetByIdentity(ctx, db, identity)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrExpenseNotFound))
}

func (s *ExpenseRepositorySuite) TestInsertDuplicateConflict() {
	mgr := setupTestDB(s.T())
	repo := newExpenseRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, db, expense))

	expense2 := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")
	err := repo.Insert(ctx, db, expense2)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrExpenseConflict))
}

func (s *ExpenseRepositorySuite) TestUpdate() {
	mgr := setupTestDB(s.T())
	repo := newExpenseRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, db, expense))

	subcategoryID := uuid.New()
	rootSlug := mustRootSlug(s.T(), "expense.conhecimento")
	competence := mustCompetence(s.T(), "2025-01")
	now := time.Now().UTC()
	s.Require().NoError(expense.Edit(subcategoryID, rootSlug, competence, 20000, now, 1, now))

	s.Require().NoError(repo.Update(ctx, db, expense, 1))

	identity := expense.Identity()
	found, _, err := repo.GetByIdentity(ctx, db, identity)
	s.Require().NoError(err)
	s.Assert().Equal(int64(2), found.Version())
	s.Assert().Equal(int64(20000), found.AmountCents())
}

func (s *ExpenseRepositorySuite) TestUpdateVersionConflict() {
	mgr := setupTestDB(s.T())
	repo := newExpenseRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, db, expense))

	subcategoryID := uuid.New()
	rootSlug := mustRootSlug(s.T(), "expense.conhecimento")
	competence := mustCompetence(s.T(), "2025-01")
	now := time.Now().UTC()
	s.Require().NoError(expense.Edit(subcategoryID, rootSlug, competence, 20000, now, 1, now))

	err := repo.Update(ctx, db, expense, 99)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrExpenseConflict))
}

func (s *ExpenseRepositorySuite) TestSoftDelete() {
	mgr := setupTestDB(s.T())
	repo := newExpenseRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, db, expense))

	now := time.Now().UTC()
	tombstoneVersion, softDeleteErr := expense.SoftDelete(1, now)
	s.Require().NoError(softDeleteErr)

	persistedTombstoneVersion, err := repo.SoftDelete(ctx, db, expense, 1)
	s.Require().NoError(err)
	s.Assert().Equal(tombstoneVersion, persistedTombstoneVersion)

	identity := expense.Identity()
	found, tombstone, err := repo.GetByIdentity(ctx, db, identity)
	s.Require().NoError(err)
	s.Assert().True(found.IsDeleted())
	s.Assert().True(tombstone.IsPresent())
	s.Assert().Equal(tombstoneVersion, tombstone.TombstoneVersion())
}

func (s *ExpenseRepositorySuite) TestSoftDeleteVersionConflict() {
	mgr := setupTestDB(s.T())
	repo := newExpenseRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, db, expense))

	now := time.Now().UTC()
	_, softDeleteErr := expense.SoftDelete(1, now)
	s.Require().NoError(softDeleteErr)

	_, err := repo.SoftDelete(ctx, db, expense, 99)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrExpenseConflict))
}

func (s *ExpenseRepositorySuite) TestTombstoneBlocksReuse() {
	mgr := setupTestDB(s.T())
	repo := newExpenseRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	userID := uuid.New()
	extID := newUUIDv4()
	expense := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")

	s.Require().NoError(repo.Insert(ctx, db, expense))

	now := time.Now().UTC()
	_, softDeleteErr := expense.SoftDelete(1, now)
	s.Require().NoError(softDeleteErr)
	_, err := repo.SoftDelete(ctx, db, expense, 1)
	s.Require().NoError(err)

	expense2 := newTestExpense(s.T(), userID, "api", extID, "2025-01", "expense.custo_fixo")
	err = repo.Insert(ctx, db, expense2)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrExpenseConflict))
}

func (s *ExpenseRepositorySuite) TestSumByRoot() {
	mgr := setupTestDB(s.T())
	repo := newExpenseRepo(testO11y())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

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
		s.Require().NoError(repo.Insert(ctx, db, e))
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
	s.Require().NoError(repo.Insert(ctx, db, deletedExpense))
	_, softDeleteErr := deletedExpense.SoftDelete(1, time.Now().UTC())
	s.Require().NoError(softDeleteErr)
	_, err := repo.SoftDelete(ctx, db, deletedExpense, 1)
	s.Require().NoError(err)

	sums, err := repo.SumByRoot(ctx, db, userID, competence)
	s.Require().NoError(err)

	custoFixo := mustRootSlug(s.T(), "expense.custo_fixo")
	s.Assert().Equal(int64(15000), sums[custoFixo])
	_, hasDeleted := sums[mustRootSlug(s.T(), "expense.conhecimento")]
	s.Assert().False(hasDeleted)
}

func (s *ExpenseRepositorySuite) TestSumByRootExplainUsesIndex() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	userID := uuid.New()
	competence := mustCompetence(s.T(), "2025-03")

	explainQuery := `EXPLAIN (FORMAT TEXT) SELECT root_slug, SUM(amount_cents) FROM mecontrola.budgets_expenses WHERE user_id = $1 AND competence = $2 AND deleted_at IS NULL GROUP BY root_slug`

	rows, err := db.QueryContext(ctx, explainQuery, userID, competence.String())
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
