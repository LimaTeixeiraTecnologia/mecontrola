//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/factories"
)

type UUIDv5NamespaceSuite struct {
	suite.Suite
}

func TestUUIDv5NamespaceSuite(t *testing.T) {
	suite.Run(t, new(UUIDv5NamespaceSuite))
}

func (s *UUIDv5NamespaceSuite) TestSeedIDsAreDeterministicRecomputable() {
	mgr := setupTestDB(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	rows, err := db.QueryContext(ctx, `
		SELECT id, kind, slug FROM mecontrola.categories
	`)
	s.Require().NoError(err)
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			s.T().Logf("close rows: %v", cerr)
		}
	}()

	checked := 0
	for rows.Next() {
		var persistedID uuid.UUID
		var kind, slug string
		s.Require().NoError(rows.Scan(&persistedID, &kind, &slug))

		recomputed := factories.NewCategoryID(kind, slug)
		s.Equalf(persistedID.String(), recomputed.String(),
			"UUIDv5 drift detected for (kind=%s, slug=%s): persisted=%s recomputed=%s",
			kind, slug, persistedID, recomputed)
		checked++
	}
	s.Require().NoError(rows.Err())
	s.Greaterf(checked, 100, "expected at least 100 categories in seed; got %d", checked)
}
