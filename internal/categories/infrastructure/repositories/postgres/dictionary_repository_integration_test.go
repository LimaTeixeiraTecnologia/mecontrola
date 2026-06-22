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

func (s *DictionaryRepositoryIntegrationSuite) TestSearchOrdersBySignalTypePrecedence() {
	ctx := context.Background()

	var catID1, catID2 uuid.UUID
	rows, err := s.db.QueryContext(ctx,
		"SELECT id FROM mecontrola.categories WHERE kind = 'expense' AND parent_id IS NOT NULL ORDER BY id LIMIT 2",
	)
	s.Require().NoError(err)
	defer rows.Close()
	s.Require().True(rows.Next())
	s.Require().NoError(rows.Scan(&catID1))
	s.Require().True(rows.Next(), "seed precisa ter 2+ subcategorias expense")
	s.Require().NoError(rows.Scan(&catID2))

	id1, id2 := uuid.New(), uuid.New()
	term := "precedence-ordering-" + id1.String()[:8]

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence)
		 VALUES ($1, $2, 'expense', $5, 'segment', 'high'),
		        ($3, $4, 'expense', $5, 'canonical_name', 'high')`,
		id1, catID1, id2, catID2, term,
	)
	s.Require().NoError(err)

	entries, err := s.repo.Search(ctx, interfaces.DictionarySearchQuery{
		Kind:  valueobjects.KindExpense,
		Term:  term,
		Limit: 10,
	})
	s.Require().NoError(err)
	s.Require().Len(entries, 2)

	for i := 1; i < len(entries); i++ {
		prev := entries[i-1].SignalType.Precedence()
		curr := entries[i].SignalType.Precedence()
		s.Assert().GreaterOrEqualf(prev, curr,
			"entries[%d].SignalType=%s (prec=%d) deve vir antes de entries[%d].SignalType=%s (prec=%d)",
			i-1, entries[i-1].SignalType, prev, i, entries[i].SignalType, curr,
		)
	}
}

func (s *DictionaryRepositoryIntegrationSuite) TestSearchTokens() {
	ctx := context.Background()

	scenarios := []struct {
		name      string
		query     interfaces.DictionaryTokenSearchQuery
		expectMin int
		wantTerm  string
	}{
		{
			name:      "deve casar sinonimo mercado por token",
			query:     interfaces.DictionaryTokenSearchQuery{Kind: valueobjects.KindExpense, Tokens: []string{"netflix", "mercado"}, Limit: 50},
			expectMin: 1,
			wantTerm:  "mercado",
		},
		{
			name:      "deve casar netflix por token",
			query:     interfaces.DictionaryTokenSearchQuery{Kind: valueobjects.KindExpense, Tokens: []string{"netflix"}, Limit: 50},
			expectMin: 1,
			wantTerm:  "netflix",
		},
		{
			name:      "deve retornar vazio quando token nao existe",
			query:     interfaces.DictionaryTokenSearchQuery{Kind: valueobjects.KindExpense, Tokens: []string{"zzzznaoexiste"}, Limit: 50},
			expectMin: 0,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			entries, err := s.repo.SearchTokens(ctx, scenario.query)
			s.Require().NoError(err)
			s.Assert().GreaterOrEqual(len(entries), scenario.expectMin)
			if scenario.wantTerm != "" {
				found := false
				for _, e := range entries {
					if e.Term == scenario.wantTerm {
						found = true
					}
				}
				s.Assert().Truef(found, "esperava termo %q nos resultados", scenario.wantTerm)
			}
		})
	}
}

func (s *DictionaryRepositoryIntegrationSuite) TestSearchFuzzyToleratesTypo() {
	ctx := context.Background()

	entries, err := s.repo.SearchFuzzy(ctx, interfaces.DictionaryFuzzySearchQuery{
		Kind:          valueobjects.KindExpense,
		Tokens:        []string{"netflyx"},
		MinSimilarity: 0.4,
		Limit:         50,
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(entries)
	s.Assert().Equal("netflix", entries[0].Term)
}

func (s *DictionaryRepositoryIntegrationSuite) TestSearchFuzzyEmptyForGibberish() {
	ctx := context.Background()

	entries, err := s.repo.SearchFuzzy(ctx, interfaces.DictionaryFuzzySearchQuery{
		Kind:          valueobjects.KindExpense,
		Tokens:        []string{"qwxzkjpv"},
		MinSimilarity: 0.4,
		Limit:         50,
	})
	s.Require().NoError(err)
	s.Assert().Empty(entries)
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
