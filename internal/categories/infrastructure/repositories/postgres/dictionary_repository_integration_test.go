//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/repositories/postgres"
)

type DictionaryRepositoryIntegrationSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo interfaces.DictionaryRepository
}

func TestDictionaryRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(DictionaryRepositoryIntegrationSuite))
}

func (s *DictionaryRepositoryIntegrationSuite) SetupSuite() {
	s.db = setupTestDB(s.T())
	s.repo = postgres.NewDictionaryRepository(noop.NewProvider(), s.db)
}

func (s *DictionaryRepositoryIntegrationSuite) SetupTest() {}

func (s *DictionaryRepositoryIntegrationSuite) TestList() {
	scenarios := []struct {
		name           string
		query          interfaces.DictionaryQuery
		expectMinCount int
		expectHasMore  bool
	}{
		{
			name:           "deve listar entradas do dicionario com paginacao padrao",
			query:          interfaces.DictionaryQuery{PageSize: 50},
			expectMinCount: 1,
			expectHasMore:  true,
		},
		{
			name:           "deve listar entradas filtradas por kind",
			query:          interfaces.DictionaryQuery{Kind: ptrKind(valueobjects.KindExpense), PageSize: 50},
			expectMinCount: 1,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()

			entries, nextCursor, err := s.repo.List(ctx, scenario.query)

			s.Require().NoError(err)
			s.Assert().NotEmpty(entries)

			if scenario.query.Kind != nil {
				for _, e := range entries {
					s.Assert().Equal(*scenario.query.Kind, e.Kind)
				}
			}

			if scenario.expectHasMore {
				s.Assert().NotEmpty(nextCursor)
			}
		})
	}
}

func (s *DictionaryRepositoryIntegrationSuite) TestListPagination() {
	ctx := context.Background()

	allEntries := make([]string, 0)
	var cursor string
	pageCount := 0

	for pageCount < 5 {
		entries, nextCursor, err := s.repo.List(ctx, interfaces.DictionaryQuery{
			PageSize: 100,
			Cursor:   cursor,
		})
		s.Require().NoError(err)

		if len(entries) == 0 {
			break
		}

		for _, e := range entries {
			allEntries = append(allEntries, e.ID.String())
		}

		cursor = nextCursor
		pageCount++

		if cursor == "" {
			break
		}
	}

	seen := make(map[string]bool)
	for _, id := range allEntries {
		s.Assert().False(seen[id], "entrada %s nao deve aparecer duplicada", id)
		seen[id] = true
	}
}

func (s *DictionaryRepositoryIntegrationSuite) TestSearch() {
	scenarios := []struct {
		name            string
		query           interfaces.DictionarySearchQuery
		expectCount     int
		expectTerm      string
		expectKind      valueobjects.Kind
		expectAmbiguous bool
	}{
		{
			name:            "deve encontrar termo agua buscando por agua",
			query:           interfaces.DictionarySearchQuery{Kind: valueobjects.KindExpense, Term: "agua", Limit: 10},
			expectCount:     1,
			expectTerm:      "agua",
			expectKind:      valueobjects.KindExpense,
			expectAmbiguous: false,
		},
		{
			name:        "deve encontrar termo agua buscando por agua com acento",
			query:       interfaces.DictionarySearchQuery{Kind: valueobjects.KindExpense, Term: "água", Limit: 10},
			expectCount: 1,
			expectTerm:  "agua",
			expectKind:  valueobjects.KindExpense,
		},
		{
			name:        "deve encontrar termo energia",
			query:       interfaces.DictionarySearchQuery{Kind: valueobjects.KindExpense, Term: "energia", Limit: 10},
			expectCount: 1,
			expectTerm:  "energia",
			expectKind:  valueobjects.KindExpense,
		},
		{
			name:        "deve retornar vazio para termo inexistente",
			query:       interfaces.DictionarySearchQuery{Kind: valueobjects.KindExpense, Term: "xyz123naoexiste", Limit: 10},
			expectCount: 0,
		},
		{
			name:        "deve filtrar por kind - termo existe em expense mas nao em income",
			query:       interfaces.DictionarySearchQuery{Kind: valueobjects.KindIncome, Term: "energia", Limit: 10},
			expectCount: 0,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()

			entries, err := s.repo.Search(ctx, scenario.query)

			s.Require().NoError(err)
			s.Assert().Len(entries, scenario.expectCount)

			if scenario.expectCount > 0 {
				s.Assert().Equal(scenario.expectTerm, entries[0].Term)
				s.Assert().Equal(scenario.expectKind, entries[0].Kind)
				s.Assert().Equal(scenario.expectAmbiguous, entries[0].IsAmbiguous)
			}
		})
	}
}

func (s *DictionaryRepositoryIntegrationSuite) TestSearchIgnoresDeprecated() {
	ctx := context.Background()

	entries, err := s.repo.Search(ctx, interfaces.DictionarySearchQuery{
		Kind:  valueobjects.KindExpense,
		Term:  "agua",
		Limit: 10,
	})
	s.Require().NoError(err)

	for _, e := range entries {
		s.Assert().Nil(e.DeprecatedAt, "entradas depreciadas nao devem aparecer na busca")
	}
}

func ptrKind(k valueobjects.Kind) *valueobjects.Kind {
	return &k
}

func ptrUUID(u uuid.UUID) *uuid.UUID {
	return &u
}

func ptrString(s string) *string {
	return &s
}
