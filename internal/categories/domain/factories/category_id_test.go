package factories_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/factories"
)

type CategoryIDSuite struct {
	suite.Suite
}

func TestCategoryIDSuite(t *testing.T) {
	suite.Run(t, new(CategoryIDSuite))
}

func (s *CategoryIDSuite) TestNewCategoryID() {
	scenarios := []struct {
		name     string
		kind     string
		slug     string
		expected uuid.UUID
	}{
		{
			name:     "deve gerar ID deterministico para expense/aluguel",
			kind:     "expense",
			slug:     "aluguel",
			expected: uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		},
		{
			name: "deve gerar ID deterministico para income/salario",
			kind: "income",
			slug: "salario",
		},
		{
			name: "deve gerar ID deterministico para expense/supermercado",
			kind: "expense",
			slug: "supermercado",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			id1 := factories.NewCategoryID(scenario.kind, scenario.slug)
			id2 := factories.NewCategoryID(scenario.kind, scenario.slug)

			s.Equal(id1, id2, "ID deve ser deterministico")
			s.NotEqual(uuid.Nil, id1, "ID nao deve ser nil")
		})
	}
}

func (s *CategoryIDSuite) TestNewCategoryIDDifferentInputs() {
	id1 := factories.NewCategoryID("expense", "aluguel")
	id2 := factories.NewCategoryID("expense", "supermercado")
	id3 := factories.NewCategoryID("income", "aluguel")

	s.NotEqual(id1, id2, "slugs diferentes devem gerar IDs diferentes")
	s.NotEqual(id1, id3, "kinds diferentes devem gerar IDs diferentes")
	s.NotEqual(id2, id3, "kinds e slugs diferentes devem gerar IDs diferentes")
}

func (s *CategoryIDSuite) TestNewCategoryID_MatchesPublishedSeedIDs() {
	expected := map[string]string{
		"expense:custo-fixo":                "66cb85a0-3266-5900-b8e3-13cdcd00ab62",
		"expense:conhecimento":              "8314f021-ee9c-53b4-872f-449ac618da50",
		"expense:prazeres":                  "ac535261-4060-56ef-b2e8-57c8cc7032d1",
		"expense:metas":                     "f133508e-7dc3-58a3-96db-199d8fbd2987",
		"expense:liberdade-financeira":      "35ced21e-b436-5cea-afb9-ffd43f98a124",
		"expense:aluguel":                   "c2fda6a3-c329-52c8-81ea-771b6ea4f365",
		"expense:financiamento-imobiliario": "f9d9e5b6-1437-5204-bd64-2bd7d43583a8",
		"expense:energia":                   "36916fab-eacc-50a3-8a53-93671c335952",
	}
	for key, want := range expected {
		var kind, slug string
		for i, r := range key {
			if r == ':' {
				kind = key[:i]
				slug = key[i+1:]
				break
			}
		}
		got := factories.NewCategoryID(kind, slug)
		s.Equalf(want, got.String(),
			"namespace drift: NewCategoryID(%q, %q) = %s, expected %s (published in migrations/000005)",
			kind, slug, got, want)
	}
}
