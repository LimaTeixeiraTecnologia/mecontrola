package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/http/server/handlers"
)

type mockListCategoriesUseCase struct{}

func (m *mockListCategoriesUseCase) Execute(ctx context.Context, in *input.ListCategoriesInput) (*output.ListCategoriesOutput, error) {
	return &output.ListCategoriesOutput{
		Categories: []output.CategoryTreeOutput{},
		Version:    1,
	}, nil
}

type mockGetCategoryUseCase struct{}

func (m *mockGetCategoryUseCase) Execute(ctx context.Context, in *input.GetCategoryInput) (*output.CategoryDetailOutput, error) {
	return &output.CategoryDetailOutput{
		ID:      in.ID,
		Name:    "Test Category",
		Version: 1,
	}, nil
}

type mockListDictionaryUseCase struct{}

func (m *mockListDictionaryUseCase) Execute(ctx context.Context, in *input.ListDictionaryInput) (*output.ListDictionaryOutput, error) {
	return &output.ListDictionaryOutput{
		Entries: []output.DictionaryEntryOutput{},
		Version: 1,
	}, nil
}

type mockSearchDictionaryUseCase struct{}

func (m *mockSearchDictionaryUseCase) Execute(ctx context.Context, in *input.SearchDictionaryInput) (*output.DictionarySearchOutput, error) {
	return &output.DictionarySearchOutput{
		Result:  "no_match",
		Version: 1,
	}, nil
}

type CategoryRouterSuite struct {
	suite.Suite
	router *CategoryRouter
}

func (s *CategoryRouterSuite) SetupTest() {
	listCategoriesHandler := handlers.NewListCategoriesHandler(&mockListCategoriesUseCase{}, nil, noop.NewProvider())
	getCategoryHandler := handlers.NewGetCategoryHandler(&mockGetCategoryUseCase{}, nil, noop.NewProvider())
	listDictionaryHandler := handlers.NewListDictionaryHandler(&mockListDictionaryUseCase{}, nil, noop.NewProvider())
	searchDictionaryHandler := handlers.NewSearchDictionaryHandler(&mockSearchDictionaryUseCase{}, nil, noop.NewProvider())

	s.router = NewCategoryRouter(
		listCategoriesHandler,
		getCategoryHandler,
		listDictionaryHandler,
		searchDictionaryHandler,
		func(next http.Handler) http.Handler { return next },
	)
}

func TestCategoryRouterSuite(t *testing.T) {
	suite.Run(t, new(CategoryRouterSuite))
}

func (s *CategoryRouterSuite) TestRegister_RequireUserMiddleware() {
	r := chi.NewRouter()
	s.router.Register(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
}

func (s *CategoryRouterSuite) TestRegister_RoutesExist() {
	r := chi.NewRouter()
	s.router.Register(r)

	testCases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/categories"},
		{http.MethodGet, "/api/v1/categories/123"},
		{http.MethodGet, "/api/v1/category-dictionary"},
		{http.MethodGet, "/api/v1/category-dictionary/search"},
	}

	for _, tc := range testCases {
		s.Run(tc.path, func() {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			s.NotEqual(http.StatusNotFound, rec.Code, "route %s should exist", tc.path)
		})
	}
}
