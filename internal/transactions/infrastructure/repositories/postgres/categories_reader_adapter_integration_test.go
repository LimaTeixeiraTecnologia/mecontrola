//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	catrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/config"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type CategoriesReaderAdapterIntegrationSuite struct {
	suite.Suite
	db      *sqlx.DB
	adapter config.CategoriesReader
}

func TestCategoriesReaderAdapterIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CategoriesReaderAdapterIntegrationSuite))
}

func (s *CategoriesReaderAdapterIntegrationSuite) SetupSuite() {
	s.db, _ = testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	categoryRepo := catrepo.NewCategoryRepository(o11y, s.db)
	versionReader := catrepo.NewVersionReader(o11y, s.db)
	resolveUC := catusecases.NewResolveBySlug(categoryRepo, o11y)
	validateUC := catusecases.NewValidateSubcategory(categoryRepo, o11y)
	s.adapter = txpostgres.NewCategoriesReaderAdapter(resolveUC, validateUC, versionReader, o11y)
}

func (s *CategoriesReaderAdapterIntegrationSuite) TestResolveOfficialRoots() {
	result, err := s.adapter.ResolveRootsBySlug(context.Background(), config.OfficialRootSlugs)

	s.Require().NoError(err)
	s.Require().Len(result, len(config.OfficialRootSlugs))
}

func (s *CategoriesReaderAdapterIntegrationSuite) TestEditorialVersion() {
	v, err := s.adapter.EditorialVersion(context.Background())

	s.Require().NoError(err)
	s.GreaterOrEqual(v, int64(0))
}

func (s *CategoriesReaderAdapterIntegrationSuite) TestResolveAndValidateFullFields() {
	ctx := context.Background()

	roots, err := s.adapter.ResolveRootsBySlug(ctx, []string{"expense.custo_fixo"})
	s.Require().NoError(err)
	rootID, ok := roots["expense.custo_fixo"]
	s.Require().True(ok)
	s.Require().NotEqual(uuid.Nil, rootID)

	var leafIDStr string
	s.Require().NoError(s.db.QueryRowContext(ctx,
		`SELECT id::text FROM mecontrola.categories WHERE parent_id=$1::uuid AND deprecated_at IS NULL LIMIT 1`,
		rootID,
	).Scan(&leafIDStr))
	leafID := uuid.MustParse(leafIDStr)

	snapshot, err := s.adapter.ValidateSubcategory(ctx, leafID, rootID)
	s.Require().NoError(err)
	s.Equal(leafID, snapshot.ID)
	s.Equal("expense", snapshot.Kind)
	s.NotEmpty(snapshot.Name)
	s.Require().NotNil(snapshot.ParentID)
	s.Equal(rootID, *snapshot.ParentID)
	s.NotEmpty(snapshot.ParentName)

	version, err := s.adapter.EditorialVersion(ctx)
	s.Require().NoError(err)
	s.Greater(version, int64(0))
}
