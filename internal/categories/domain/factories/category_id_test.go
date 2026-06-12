package factories_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/factories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type CategoryIDSuite struct {
	suite.Suite
}

func TestCategoryIDSuite(t *testing.T) {
	suite.Run(t, new(CategoryIDSuite))
}

func (s *CategoryIDSuite) newSlug(raw string) valueobjects.Slug {
	slug, err := valueobjects.NewSlug(raw)
	s.Require().NoError(err)
	return slug
}

func (s *CategoryIDSuite) TestNewCategoryID() {
	scenarios := []struct {
		name string
		kind string
		slug string
	}{
		{name: "deve gerar ID deterministico para expense/aluguel", kind: "expense", slug: "aluguel"},
		{name: "deve gerar ID deterministico para income/salario", kind: "income", slug: "salario"},
		{name: "deve gerar ID deterministico para expense/supermercado", kind: "expense", slug: "supermercado"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			slug := s.newSlug(scenario.slug)
			id1 := factories.NewCategoryID(scenario.kind, slug)
			id2 := factories.NewCategoryID(scenario.kind, slug)

			s.Equal(id1, id2, "ID deve ser deterministico")
			s.NotEqual(uuid.Nil, id1, "ID nao deve ser nil")
		})
	}
}

func (s *CategoryIDSuite) TestNewCategoryIDDifferentInputs() {
	id1 := factories.NewCategoryID("expense", s.newSlug("aluguel"))
	id2 := factories.NewCategoryID("expense", s.newSlug("supermercado"))
	id3 := factories.NewCategoryID("income", s.newSlug("aluguel"))

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
		parts := strings.SplitN(key, ":", 2)
		s.Require().Len(parts, 2)
		kind, slugRaw := parts[0], parts[1]
		slug := s.newSlug(slugRaw)
		got := factories.NewCategoryID(kind, slug)
		s.Equalf(want, got.String(),
			"namespace drift: NewCategoryID(%q, %q) = %s, expected %s (published in migrations/000005)",
			kind, slugRaw, got, want)
	}
}
