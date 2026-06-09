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
