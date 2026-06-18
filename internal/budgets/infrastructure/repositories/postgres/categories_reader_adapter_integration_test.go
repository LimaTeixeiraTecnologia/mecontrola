//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/jmoiron/sqlx"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	budgetsinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	budgetsconfig "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/config"
	budgetspostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories/postgres"
	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	catrepository "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type CategoriesReaderAdapterIntegrationSuite struct {
	suite.Suite
	db      *sqlx.DB
	adapter budgetsinterfaces.CategoriesReader
}

func TestCategoriesReaderAdapterIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CategoriesReaderAdapterIntegrationSuite))
}

func (s *CategoriesReaderAdapterIntegrationSuite) SetupSuite() {
	s.db, _ = testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	db := s.db
	categoryRepo := catrepository.NewCategoryRepository(o11y, db)
	versionReader := catrepository.NewVersionReader(o11y, db)
	resolveUC := catusecases.NewResolveBySlug(categoryRepo, o11y)
	validateUC := catusecases.NewValidateSubcategory(categoryRepo, o11y)
	s.adapter = budgetspostgres.NewCategoriesReaderAdapter(resolveUC, validateUC, versionReader, o11y)
}

func (s *CategoriesReaderAdapterIntegrationSuite) TestResolveOfficialRoots() {
	result, err := s.adapter.ResolveRootsBySlug(context.Background(), budgetsconfig.OfficialRootSlugs)

	s.Require().NoError(err)
	s.Require().Len(result, len(budgetsconfig.OfficialRootSlugs))
	for _, slug := range budgetsconfig.OfficialRootSlugs {
		id, ok := result[slug]
		s.True(ok, "slug %q deve estar presente", slug)
		s.NotEqual(uuid.Nil, id, "id para %q não deve ser nil", slug)
	}
}

func (s *CategoriesReaderAdapterIntegrationSuite) TestValidateActiveSubcategory() {
	custoFixoSubcategoryID := uuid.MustParse("d1d7dbba-1e83-596c-a4e5-d520cd06c88a")

	rootSlug, deprecated, err := s.adapter.ValidateExpenseSubcategory(context.Background(), custoFixoSubcategoryID)

	s.Require().NoError(err)
	s.Equal("expense.custo_fixo", rootSlug)
	s.False(deprecated)
}

func (s *CategoriesReaderAdapterIntegrationSuite) TestValidateNonExistentSubcategory() {
	nonExistentID := uuid.New()

	_, _, err := s.adapter.ValidateExpenseSubcategory(context.Background(), nonExistentID)

	s.Require().Error(err)
	s.True(errors.Is(err, budgetsinterfaces.ErrCategoriesReaderUnavailable))
}

func (s *CategoriesReaderAdapterIntegrationSuite) TestEditorialVersion() {
	v, err := s.adapter.EditorialVersion(context.Background())

	s.Require().NoError(err)
	s.GreaterOrEqual(v, int64(0))
}
