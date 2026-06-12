//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	catrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/config"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type CategoriesReaderAdapterIntegrationSuite struct {
	suite.Suite
	mgr     manager.Manager
	adapter config.CategoriesReader
}

func TestCategoriesReaderAdapterIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CategoriesReaderAdapterIntegrationSuite))
}

func (s *CategoriesReaderAdapterIntegrationSuite) SetupSuite() {
	s.mgr, _ = testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	db := s.mgr.DBTX(context.Background())
	categoryRepo := catrepo.NewCategoryRepository(o11y, db)
	versionReader := catrepo.NewVersionReader(o11y, db)
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
