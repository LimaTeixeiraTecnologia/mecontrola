package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/config"
)

type CategoriesCacheSuite struct {
	suite.Suite
}

func TestCategoriesCacheSuite(t *testing.T) {
	suite.Run(t, new(CategoriesCacheSuite))
}

func (s *CategoriesCacheSuite) TestBoot_Success() {
	reader := mocks.NewCategoriesReader(s.T())
	roots := make(map[string]uuid.UUID, len(config.OfficialRootSlugs))
	for _, slug := range config.OfficialRootSlugs {
		roots[slug] = uuid.New()
	}

	reader.EXPECT().ResolveRootsBySlug(context.Background(), config.OfficialRootSlugs).Return(roots, nil)
	reader.EXPECT().EditorialVersion(context.Background()).Return(int64(1), nil)

	cache := config.NewCategoriesCache(reader)

	err := cache.Boot(context.Background())
	s.Require().NoError(err)

	for _, slug := range config.OfficialRootSlugs {
		id, ok := cache.RootID(slug)
		s.True(ok, "slug %q deve estar em cache após boot", slug)
		s.NotEqual(uuid.Nil, id)
	}
}

func (s *CategoriesCacheSuite) TestBoot_ResolveError() {
	reader := mocks.NewCategoriesReader(s.T())

	reader.EXPECT().ResolveRootsBySlug(context.Background(), config.OfficialRootSlugs).Return(nil, errors.New("categories unavailable"))

	cache := config.NewCategoriesCache(reader)

	err := cache.Boot(context.Background())
	s.Require().Error(err)
}

func (s *CategoriesCacheSuite) TestBoot_InsufficientRoots() {
	reader := mocks.NewCategoriesReader(s.T())
	roots := map[string]uuid.UUID{
		"expense.custo_fixo": uuid.New(),
	}

	reader.EXPECT().ResolveRootsBySlug(context.Background(), config.OfficialRootSlugs).Return(roots, nil)

	cache := config.NewCategoriesCache(reader)

	err := cache.Boot(context.Background())
	s.Require().Error(err)
}

func (s *CategoriesCacheSuite) TestValidateExpenseSubcategory_CacheHit() {
	reader := mocks.NewCategoriesReader(s.T())
	roots := make(map[string]uuid.UUID, len(config.OfficialRootSlugs))
	for _, slug := range config.OfficialRootSlugs {
		roots[slug] = uuid.New()
	}

	subID := uuid.New()

	reader.EXPECT().ResolveRootsBySlug(context.Background(), config.OfficialRootSlugs).Return(roots, nil)
	reader.EXPECT().EditorialVersion(context.Background()).Return(int64(1), nil).Times(3)
	reader.EXPECT().ValidateExpenseSubcategory(context.Background(), subID).Return("expense.custo_fixo", false, nil).Once()

	cache := config.NewCategoriesCache(reader)
	s.Require().NoError(cache.Boot(context.Background()))

	rootSlug1, dep1, err1 := cache.ValidateExpenseSubcategory(context.Background(), subID)
	s.Require().NoError(err1)
	s.Equal("expense.custo_fixo", rootSlug1)
	s.False(dep1)

	rootSlug2, dep2, err2 := cache.ValidateExpenseSubcategory(context.Background(), subID)
	s.Require().NoError(err2)
	s.Equal("expense.custo_fixo", rootSlug2)
	s.False(dep2)
}

func (s *CategoriesCacheSuite) TestValidateExpenseSubcategory_CacheBustOnVersionChange() {
	reader := mocks.NewCategoriesReader(s.T())
	roots := make(map[string]uuid.UUID, len(config.OfficialRootSlugs))
	for _, slug := range config.OfficialRootSlugs {
		roots[slug] = uuid.New()
	}

	subID := uuid.New()

	reader.EXPECT().ResolveRootsBySlug(context.Background(), config.OfficialRootSlugs).Return(roots, nil)
	reader.EXPECT().EditorialVersion(context.Background()).Return(int64(1), nil).Once()
	reader.EXPECT().EditorialVersion(context.Background()).Return(int64(1), nil).Once()
	reader.EXPECT().ValidateExpenseSubcategory(context.Background(), subID).Return("expense.custo_fixo", false, nil).Once()
	reader.EXPECT().EditorialVersion(context.Background()).Return(int64(2), nil).Once()
	reader.EXPECT().ValidateExpenseSubcategory(context.Background(), subID).Return("expense.conhecimento", false, nil).Once()

	cache := config.NewCategoriesCache(reader)
	s.Require().NoError(cache.Boot(context.Background()))

	_, _, err1 := cache.ValidateExpenseSubcategory(context.Background(), subID)
	s.Require().NoError(err1)

	rootSlug2, _, err2 := cache.ValidateExpenseSubcategory(context.Background(), subID)
	s.Require().NoError(err2)
	s.Equal("expense.conhecimento", rootSlug2)
}

func (s *CategoriesCacheSuite) TestIsAllowedProducerSource_Allowed() {
	s.False(config.IsAllowedProducerSource("some-producer"))
}

func (s *CategoriesCacheSuite) TestIsAllowedProducerSource_Empty() {
	s.False(config.IsAllowedProducerSource(""))
}
