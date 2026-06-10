//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type SchemaRegressionSuite struct {
	suite.Suite
}

func TestSchemaRegressionSuite(t *testing.T) {
	suite.Run(t, new(SchemaRegressionSuite))
}

func (s *SchemaRegressionSuite) TestParentSameKindTrigger_RejectsCrossKindParent() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	var parentID uuid.UUID
	err := db.QueryRowContext(ctx, `
		SELECT id FROM mecontrola.categories
		WHERE kind = 'expense' AND parent_id IS NULL
		LIMIT 1
	`).Scan(&parentID)
	s.Require().NoError(err, "seed must provide at least one expense root")

	childID := uuid.New()
	_, err = db.ExecContext(ctx, `
		INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, childID, "regression-cross-kind-"+childID.String(), "Regression Cross Kind", "income", parentID, "consumption")

	s.Require().Error(err, "expected trigger to reject income child of expense parent")
	s.Contains(err.Error(), "categories_parent_same_kind")
}

func (s *SchemaRegressionSuite) TestParentSortIndex_UsesPTBRCollation() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	var indexDef string
	err := db.QueryRowContext(ctx, `
		SELECT indexdef FROM pg_indexes
		WHERE schemaname = 'mecontrola' AND indexname = 'categories_parent_sort_idx'
	`).Scan(&indexDef)
	s.Require().NoError(err)
	s.Contains(indexDef, `"pt-BR-x-icu"`, "categories_parent_sort_idx must use COLLATE pt-BR-x-icu for RF-11")
}

func (s *SchemaRegressionSuite) TestDictionaryIndexes_UsePTBRCollation() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	for _, idx := range []string{"dictionary_term_normalized_idx", "dictionary_kind_term_normalized_idx"} {
		var indexDef string
		err := db.QueryRowContext(ctx, `
			SELECT indexdef FROM pg_indexes
			WHERE schemaname = 'mecontrola' AND indexname = $1
		`, idx).Scan(&indexDef)
		s.Require().NoErrorf(err, "index %s not found", idx)
		s.Containsf(indexDef, `"pt-BR-x-icu"`, "%s must use COLLATE pt-BR-x-icu for cursor pagination coherence (RF-14a)", idx)
	}
}

func (s *SchemaRegressionSuite) TestParentKindChange_BlocksWhenChildrenExist() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	var rootID uuid.UUID
	err := db.QueryRowContext(ctx, `
		SELECT id FROM mecontrola.categories
		WHERE kind = 'expense' AND parent_id IS NULL
		LIMIT 1
	`).Scan(&rootID)
	s.Require().NoError(err)

	_, err = db.ExecContext(ctx, `UPDATE mecontrola.categories SET kind = 'income' WHERE id = $1`, rootID)
	s.Require().Error(err, "trigger must block kind change on root with active children")
	s.Contains(err.Error(), "categories_parent_kind_change_blocks_children")
}
