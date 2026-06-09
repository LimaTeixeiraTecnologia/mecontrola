package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type CategorySuite struct {
	suite.Suite
}

func TestCategorySuite(t *testing.T) {
	suite.Run(t, new(CategorySuite))
}

func (s *CategorySuite) TestCategoryIsRoot() {
	parentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	scenarios := []struct {
		name string
		cat  entities.Category
		want bool
	}{
		{
			name: "categoria com ParentID nil eh raiz",
			cat:  entities.Category{ID: uuid.New(), ParentID: nil},
			want: true,
		},
		{
			name: "categoria com ParentID preenchido nao eh raiz",
			cat:  entities.Category{ID: uuid.New(), ParentID: &parentID},
			want: false,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.cat.IsRoot())
		})
	}
}

func (s *CategorySuite) TestCategoryIsActive() {
	now := time.Now()

	scenarios := []struct {
		name string
		cat  entities.Category
		want bool
	}{
		{
			name: "categoria sem DeprecatedAt eh ativa",
			cat:  entities.Category{DeprecatedAt: nil},
			want: true,
		},
		{
			name: "categoria com DeprecatedAt preenchido nao eh ativa",
			cat:  entities.Category{DeprecatedAt: &now},
			want: false,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.cat.IsActive())
		})
	}
}

func (s *CategorySuite) TestCategoryFields() {
	id := uuid.New()
	parentID := uuid.New()
	now := time.Now()

	cat := entities.Category{
		ID:             id,
		Slug:           "aluguel",
		Name:           "Aluguel",
		Kind:           valueobjects.KindExpense,
		ParentID:       &parentID,
		AllocationType: valueobjects.AllocationTypeConsumption,
		DeprecatedAt:   &now,
	}

	s.Equal(id, cat.ID)
	s.Equal("aluguel", cat.Slug)
	s.Equal("Aluguel", cat.Name)
	s.Equal(valueobjects.KindExpense, cat.Kind)
	s.Equal(&parentID, cat.ParentID)
	s.Equal(valueobjects.AllocationTypeConsumption, cat.AllocationType)
	s.Equal(&now, cat.DeprecatedAt)
}
