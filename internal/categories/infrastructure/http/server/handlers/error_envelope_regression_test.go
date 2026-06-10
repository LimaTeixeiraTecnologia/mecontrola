package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
)

type stubVersionReader struct {
	value int64
	err   error
}

func (s *stubVersionReader) Current(ctx context.Context) (int64, error) {
	return s.value, s.err
}

type erroringGetCategoryUC struct{}

func (e *erroringGetCategoryUC) Execute(ctx context.Context, in *input.GetCategoryInput) (*output.CategoryDetailOutput, error) {
	return nil, usecases.ErrCategoryNotFound
}

type erroringListCategoriesUC struct{}

func (e *erroringListCategoriesUC) Execute(ctx context.Context, in *input.ListCategoriesInput) (*output.ListCategoriesOutput, error) {
	return nil, errors.New("boom")
}

type erroringListDictionaryUC struct{}

func (e *erroringListDictionaryUC) Execute(ctx context.Context, in *input.ListDictionaryInput) (*output.ListDictionaryOutput, error) {
	return nil, errors.New("boom")
}

type ErrorEnvelopeRegressionSuite struct {
	suite.Suite
}

func TestErrorEnvelopeRegressionSuite(t *testing.T) {
	suite.Run(t, new(ErrorEnvelopeRegressionSuite))
}

func (s *ErrorEnvelopeRegressionSuite) decode(body []byte) map[string]any {
	var m map[string]any
	require.NoError(s.T(), json.Unmarshal(body, &m))
	return m
}

func (s *ErrorEnvelopeRegressionSuite) TestGetCategory_NotFound_IncludesVersionAndETag() {
	h := NewGetCategoryHandler(&erroringGetCategoryUC{}, &stubVersionReader{value: 42}, noop.NewProvider())
	id := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/categories/"+id, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal(`"v42"`, rec.Header().Get("ETag"))
	body := s.decode(rec.Body.Bytes())
	s.EqualValues(42, body["version"])
	errs, ok := body["errors"].(map[string]any)
	s.Require().True(ok, "errors object must exist")
	s.Equal("not_found", errs["code"])
}

func (s *ErrorEnvelopeRegressionSuite) TestGetCategory_InvalidUUID_IncludesVersion() {
	h := NewGetCategoryHandler(&erroringGetCategoryUC{}, &stubVersionReader{value: 7}, noop.NewProvider())
	req := httptest.NewRequest(http.MethodGet, "/categories/not-a-uuid", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Equal(`"v7"`, rec.Header().Get("ETag"))
	body := s.decode(rec.Body.Bytes())
	s.EqualValues(7, body["version"])
	errs := body["errors"].(map[string]any)
	s.Equal("invalid_query", errs["code"])
}

func (s *ErrorEnvelopeRegressionSuite) TestSearchDictionary_InvalidKind_IncludesVersion() {
	mockUC := new(mockSearchDictionaryUseCase)
	h := NewSearchDictionaryHandler(mockUC, &stubVersionReader{value: 99}, noop.NewProvider())
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=energia&kind=foo", nil)
	rec := httptest.NewRecorder()

	h.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Equal(`"v99"`, rec.Header().Get("ETag"))
	body := s.decode(rec.Body.Bytes())
	s.EqualValues(99, body["version"])
	errs := body["errors"].(map[string]any)
	s.Equal("invalid_kind", errs["code"])
}

func (s *ErrorEnvelopeRegressionSuite) TestSearchDictionary_KindMissing_IncludesVersion() {
	mockUC := new(mockSearchDictionaryUseCase)
	h := NewSearchDictionaryHandler(mockUC, &stubVersionReader{value: 12}, noop.NewProvider())
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=energia", nil)
	rec := httptest.NewRecorder()

	h.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Equal(`"v12"`, rec.Header().Get("ETag"))
	body := s.decode(rec.Body.Bytes())
	s.EqualValues(12, body["version"])
	errs := body["errors"].(map[string]any)
	s.Equal("invalid_kind", errs["code"])
}

func (s *ErrorEnvelopeRegressionSuite) TestListCategories_InternalError_IncludesVersion() {
	h := NewListCategoriesHandler(&erroringListCategoriesUC{}, &stubVersionReader{value: 100}, noop.NewProvider())
	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	rec := httptest.NewRecorder()

	h.Handle(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal(`"v100"`, rec.Header().Get("ETag"))
	body := s.decode(rec.Body.Bytes())
	s.EqualValues(100, body["version"])
}

func (s *ErrorEnvelopeRegressionSuite) TestListDictionary_InvalidKind_IncludesVersion() {
	h := NewListDictionaryHandler(&erroringListDictionaryUC{}, &stubVersionReader{value: 5}, noop.NewProvider())
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary?kind=invalid", nil)
	rec := httptest.NewRecorder()

	h.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Equal(`"v5"`, rec.Header().Get("ETag"))
	body := s.decode(rec.Body.Bytes())
	s.EqualValues(5, body["version"])
	errs := body["errors"].(map[string]any)
	s.Equal("invalid_kind", errs["code"])
}

func (s *ErrorEnvelopeRegressionSuite) TestVersionReader_ErrorFallsBackToZero() {
	h := NewListCategoriesHandler(&erroringListCategoriesUC{}, &stubVersionReader{value: 0, err: errors.New("db down")}, noop.NewProvider())
	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	rec := httptest.NewRecorder()

	h.Handle(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	body := s.decode(rec.Body.Bytes())
	s.EqualValues(0, body["version"])
}
