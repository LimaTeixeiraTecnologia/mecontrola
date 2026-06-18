//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/repositories/postgres"
)

type CategoryRepositoryIntegrationSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo interfaces.CategoryRepository
}

func TestCategoryRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CategoryRepositoryIntegrationSuite))
}

func (s *CategoryRepositoryIntegrationSuite) SetupSuite() {
	s.db = setupTestDB(s.T())
	s.repo = postgres.NewCategoryRepository(noop.NewProvider(), s.db)
}

func (s *CategoryRepositoryIntegrationSuite) SetupTest() {}

func (s *CategoryRepositoryIntegrationSuite) TestList() {
	scenarios := []struct {
		name           string
		query          interfaces.CategoryQuery
		expectMinCount int
		expectKind     valueobjects.Kind
		expectHasRoots bool
		expectErr      error
	}{
		{
			name:           "deve listar categorias expense com raizes e subcategorias",
			query:          interfaces.CategoryQuery{Kind: valueobjects.KindExpense, IncludeDeprecated: false},
			expectMinCount: 5,
			expectKind:     valueobjects.KindExpense,
			expectHasRoots: true,
		},
		{
			name:           "deve listar categorias income com raizes e subcategorias",
			query:          interfaces.CategoryQuery{Kind: valueobjects.KindIncome, IncludeDeprecated: false},
			expectMinCount: 8,
			expectKind:     valueobjects.KindIncome,
			expectHasRoots: true,
		},
		{
			name:           "deve retornar lista vazia para kind invalido",
			query:          interfaces.CategoryQuery{Kind: 0, IncludeDeprecated: false},
			expectMinCount: 0,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()

			categories, err := s.repo.List(ctx, scenario.query)

			if scenario.expectErr != nil {
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, scenario.expectErr))
				return
			}

			s.Require().NoError(err)
			s.Assert().GreaterOrEqual(len(categories), scenario.expectMinCount)

			if scenario.expectMinCount > 0 {
				for _, c := range categories {
					s.Assert().Equal(scenario.expectKind, c.Kind)
				}
			}
		})
	}
}

func (s *CategoryRepositoryIntegrationSuite) TestListWithParentID() {
	ctx := context.Background()

	roots, err := s.repo.List(ctx, interfaces.CategoryQuery{
		Kind:              valueobjects.KindExpense,
		ParentID:          nil,
		IncludeDeprecated: false,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(roots)

	var custoFixoID *uuid.UUID
	for _, r := range roots {
		if r.Slug == "custo-fixo" {
			custoFixoID = &r.ID
			break
		}
	}
	s.Require().NotNil(custoFixoID)

	scenarios := []struct {
		name           string
		parentID       *uuid.UUID
		expectMinCount int
	}{
		{
			name:           "deve listar subcategorias de Custo Fixo",
			parentID:       custoFixoID,
			expectMinCount: 41,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			subcategories, err := s.repo.List(ctx, interfaces.CategoryQuery{
				Kind:              valueobjects.KindExpense,
				ParentID:          scenario.parentID,
				IncludeDeprecated: false,
			})

			s.Require().NoError(err)
			s.Assert().GreaterOrEqual(len(subcategories), scenario.expectMinCount)

			for _, sub := range subcategories {
				s.Assert().Equal(scenario.parentID, sub.ParentID)
			}
		})
	}
}

func (s *CategoryRepositoryIntegrationSuite) TestListOrdering() {
	ctx := context.Background()

	categories, err := s.repo.List(ctx, interfaces.CategoryQuery{
		Kind:              valueobjects.KindExpense,
		IncludeDeprecated: false,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(categories)

	col := ptBRCollator()
	for i := 1; i < len(categories); i++ {
		prev := categories[i-1].Name
		curr := categories[i].Name
		s.Assert().LessOrEqualf(col.CompareString(prev, curr), 0,
			"categorias devem estar ordenadas alfabeticamente PT-BR: %s > %s", prev, curr)
	}
}

func (s *CategoryRepositoryIntegrationSuite) TestGetByID() {
	ctx := context.Background()

	categories, err := s.repo.List(ctx, interfaces.CategoryQuery{
		Kind:              valueobjects.KindExpense,
		IncludeDeprecated: false,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(categories)

	first := categories[0]

	scenarios := []struct {
		name      string
		id        uuid.UUID
		expectID  uuid.UUID
		expectErr error
	}{
		{
			name:     "deve encontrar categoria por ID",
			id:       first.ID,
			expectID: first.ID,
		},
		{
			name:      "deve retornar erro quando categoria nao existe",
			id:        uuid.MustParse("00000000-0000-0000-0000-000000000000"),
			expectErr: postgres.ErrCategoryNotFound,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			category, err := s.repo.GetByID(ctx, scenario.id)

			if scenario.expectErr != nil {
				s.Require().Error(err)
				s.Assert().True(errors.Is(err, scenario.expectErr))
				return
			}

			s.Require().NoError(err)
			s.Assert().Equal(scenario.expectID, category.ID)
		})
	}
}

func (s *CategoryRepositoryIntegrationSuite) TestListByIDs() {
	ctx := context.Background()

	all, err := s.repo.List(ctx, interfaces.CategoryQuery{
		Kind:              valueobjects.KindExpense,
		IncludeDeprecated: false,
	})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(all), 3)

	ids := make([]uuid.UUID, 3)
	for i := range 3 {
		ids[i] = all[i].ID
	}

	result, err := s.repo.ListByIDs(ctx, ids)
	s.Require().NoError(err)
	s.Assert().Len(result, 3)

	idSet := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	for _, c := range result {
		_, ok := idSet[c.ID]
		s.Assert().True(ok, "ID %s nao estava no conjunto solicitado", c.ID)
	}

	var sqlCount int
	row := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM mecontrola.categories WHERE id IN ($1, $2, $3)",
		ids[0], ids[1], ids[2],
	)
	s.Require().NoError(row.Scan(&sqlCount))
	s.Assert().Equal(len(result), sqlCount)
}

func (s *CategoryRepositoryIntegrationSuite) TestListIncludeDeprecated() {
	ctx := context.Background()

	deprecatedID := uuid.New()
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO mecontrola.categories (id, slug, name, kind, allocation_type, deprecated_at) VALUES ($1, $2, $3, $4, $5, NOW())",
		deprecatedID, "deprecated-test-"+deprecatedID.String(), "Deprecated Test", "expense", "consumption",
	)
	s.Require().NoError(err)

	withoutDeprecated, err := s.repo.List(ctx, interfaces.CategoryQuery{
		Kind:              valueobjects.KindExpense,
		IncludeDeprecated: false,
	})
	s.Require().NoError(err)
	for _, c := range withoutDeprecated {
		s.Assert().NotEqual(deprecatedID, c.ID, "categoria deprecated nao deve aparecer quando IncludeDeprecated=false")
	}

	withDeprecated, err := s.repo.List(ctx, interfaces.CategoryQuery{
		Kind:              valueobjects.KindExpense,
		IncludeDeprecated: true,
	})
	s.Require().NoError(err)

	found := false
	for _, c := range withDeprecated {
		if c.ID == deprecatedID {
			found = true
			break
		}
	}
	s.Assert().True(found, "categoria deprecated deve aparecer quando IncludeDeprecated=true")

	var sqlCount int
	row := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM mecontrola.categories WHERE deprecated_at IS NOT NULL AND kind = 'expense'",
	)
	s.Require().NoError(row.Scan(&sqlCount))
	s.Assert().GreaterOrEqual(sqlCount, 1)
}
