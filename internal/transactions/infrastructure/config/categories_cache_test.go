package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/config"
)

type mockCategoriesReader struct {
	resolveRootsFunc        func(ctx context.Context, slugs []string) (map[string]uuid.UUID, error)
	validateSubcategoryFunc func(ctx context.Context, id uuid.UUID) (interfaces.CategorySnapshot, error)
	editorialVersionFunc    func(ctx context.Context) (int64, error)
}

func (m *mockCategoriesReader) ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error) {
	return m.resolveRootsFunc(ctx, slugs)
}

func (m *mockCategoriesReader) ValidateSubcategory(ctx context.Context, id uuid.UUID) (interfaces.CategorySnapshot, error) {
	return m.validateSubcategoryFunc(ctx, id)
}

func (m *mockCategoriesReader) EditorialVersion(ctx context.Context) (int64, error) {
	return m.editorialVersionFunc(ctx)
}

type CategoriesCacheSuite struct {
	suite.Suite
}

func TestCategoriesCacheSuite(t *testing.T) {
	suite.Run(t, new(CategoriesCacheSuite))
}

func (s *CategoriesCacheSuite) buildRoots() map[string]uuid.UUID {
	roots := make(map[string]uuid.UUID, len(config.OfficialRootSlugs)+len(config.OfficialIncomeRootSlugs))
	for _, slug := range config.OfficialRootSlugs {
		roots[slug] = uuid.New()
	}
	for _, slug := range config.OfficialIncomeRootSlugs {
		roots[slug] = uuid.New()
	}
	return roots
}

func resolveSubset(master map[string]uuid.UUID) func(context.Context, []string) (map[string]uuid.UUID, error) {
	return func(_ context.Context, slugs []string) (map[string]uuid.UUID, error) {
		out := make(map[string]uuid.UUID, len(slugs))
		for _, slug := range slugs {
			if id, ok := master[slug]; ok {
				out[slug] = id
			}
		}
		return out, nil
	}
}

func (s *CategoriesCacheSuite) TestBoot_Success() {
	roots := s.buildRoots()
	reader := &mockCategoriesReader{
		resolveRootsFunc: resolveSubset(roots),
		editorialVersionFunc: func(_ context.Context) (int64, error) {
			return 1, nil
		},
	}

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
	reader := &mockCategoriesReader{
		resolveRootsFunc: func(_ context.Context, slugs []string) (map[string]uuid.UUID, error) {
			return nil, errors.New("categories unavailable")
		},
	}

	cache := config.NewCategoriesCache(reader)
	err := cache.Boot(context.Background())
	s.Require().Error(err)
}

func (s *CategoriesCacheSuite) TestBoot_InsufficientRoots() {
	reader := &mockCategoriesReader{
		resolveRootsFunc: func(_ context.Context, slugs []string) (map[string]uuid.UUID, error) {
			return map[string]uuid.UUID{"expense.custo_fixo": uuid.New()}, nil
		},
	}

	cache := config.NewCategoriesCache(reader)
	err := cache.Boot(context.Background())
	s.Require().Error(err)
}

func (s *CategoriesCacheSuite) TestBoot_EditorialVersionError() {
	roots := s.buildRoots()
	reader := &mockCategoriesReader{
		resolveRootsFunc: resolveSubset(roots),
		editorialVersionFunc: func(_ context.Context) (int64, error) {
			return 0, errors.New("version unavailable")
		},
	}

	cache := config.NewCategoriesCache(reader)
	err := cache.Boot(context.Background())
	s.Require().Error(err)
}

func (s *CategoriesCacheSuite) TestValidate_NilSubcategoryID_RootFound() {
	roots := s.buildRoots()
	reader := &mockCategoriesReader{
		resolveRootsFunc: resolveSubset(roots),
		editorialVersionFunc: func(_ context.Context) (int64, error) {
			return 1, nil
		},
	}

	cache := config.NewCategoriesCache(reader)
	s.Require().NoError(cache.Boot(context.Background()))

	var targetID uuid.UUID
	for _, id := range roots {
		targetID = id
		break
	}

	snapshot, err := cache.Validate(context.Background(), targetID, nil)
	s.Require().NoError(err)
	s.Equal(targetID, snapshot.ID)
}

func (s *CategoriesCacheSuite) TestValidate_NilSubcategoryID_RootNotFound() {
	roots := s.buildRoots()
	reader := &mockCategoriesReader{
		resolveRootsFunc: resolveSubset(roots),
		editorialVersionFunc: func(_ context.Context) (int64, error) {
			return 1, nil
		},
	}

	cache := config.NewCategoriesCache(reader)
	s.Require().NoError(cache.Boot(context.Background()))

	unknownID := uuid.New()
	_, err := cache.Validate(context.Background(), unknownID, nil)
	s.Require().Error(err)
	s.True(errors.Is(err, interfaces.ErrCategoryNotFound))
}

func (s *CategoriesCacheSuite) TestValidate_SubcategoryID_CacheMiss_ThenCacheHit() {
	roots := s.buildRoots()
	subID := uuid.New()
	parentID := uuid.New()
	versionCallCount := 0
	subcategoryCallCount := 0

	reader := &mockCategoriesReader{
		resolveRootsFunc: resolveSubset(roots),
		editorialVersionFunc: func(_ context.Context) (int64, error) {
			versionCallCount++
			return 1, nil
		},
		validateSubcategoryFunc: func(_ context.Context, id uuid.UUID) (interfaces.CategorySnapshot, error) {
			subcategoryCallCount++
			return interfaces.CategorySnapshot{
				ID:         id,
				Name:       "sub-name",
				ParentID:   &parentID,
				ParentName: "parent-name",
			}, nil
		},
	}

	cache := config.NewCategoriesCache(reader)
	s.Require().NoError(cache.Boot(context.Background()))

	snapshot1, err := cache.Validate(context.Background(), uuid.New(), &subID)
	s.Require().NoError(err)
	s.Equal(subID, snapshot1.ID)
	s.Equal(1, subcategoryCallCount)

	snapshot2, err := cache.Validate(context.Background(), uuid.New(), &subID)
	s.Require().NoError(err)
	s.Equal(subID, snapshot2.ID)
	s.Equal(1, subcategoryCallCount, "segunda chamada deve usar o cache")
}

func (s *CategoriesCacheSuite) TestValidate_SubcategoryID_VersionChange_InvalidatesCache() {
	roots := s.buildRoots()
	subID := uuid.New()
	version := int64(1)
	subcategoryCallCount := 0

	reader := &mockCategoriesReader{
		resolveRootsFunc: resolveSubset(roots),
		editorialVersionFunc: func(_ context.Context) (int64, error) {
			return version, nil
		},
		validateSubcategoryFunc: func(_ context.Context, id uuid.UUID) (interfaces.CategorySnapshot, error) {
			subcategoryCallCount++
			return interfaces.CategorySnapshot{ID: id, Name: "sub-name"}, nil
		},
	}

	cache := config.NewCategoriesCache(reader)
	s.Require().NoError(cache.Boot(context.Background()))

	_, err := cache.Validate(context.Background(), uuid.New(), &subID)
	s.Require().NoError(err)
	s.Equal(1, subcategoryCallCount)

	version = 2

	_, err = cache.Validate(context.Background(), uuid.New(), &subID)
	s.Require().NoError(err)
	s.Equal(2, subcategoryCallCount, "nova versão deve invalidar cache e reconsultar")
}
