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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
)

type mockGetCategoryUseCase struct {
	mock.Mock
}

func (m *mockGetCategoryUseCase) Execute(ctx context.Context, in *input.GetCategoryInput) (*output.CategoryDetailOutput, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*output.CategoryDetailOutput), args.Error(1)
}

type GetCategoryHandlerSuite struct {
	suite.Suite
	handler *GetCategoryHandler
	mockUC  *mockGetCategoryUseCase
}

func (s *GetCategoryHandlerSuite) SetupTest() {
	s.mockUC = new(mockGetCategoryUseCase)
	s.handler = NewGetCategoryHandler(s.mockUC, noop.NewProvider())
}

func TestGetCategoryHandlerSuite(t *testing.T) {
	suite.Run(t, new(GetCategoryHandlerSuite))
}

func (s *GetCategoryHandlerSuite) newRequestWithID(id string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/categories/"+id, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func (s *GetCategoryHandlerSuite) TestHandle_Success() {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	req := s.newRequestWithID(id.String())
	rec := httptest.NewRecorder()

	expectedOutput := &output.CategoryDetailOutput{
		ID:      id,
		Name:    "Salario",
		Kind:    "income",
		Path:    "Salario",
		Version: 42,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.GetCategoryInput) bool {
		return in.ID == id
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal(`"v42"`, rec.Header().Get("ETag"))

	var response output.CategoryDetailOutput
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal(id, response.ID)
	s.Equal("Salario", response.Name)
}

func (s *GetCategoryHandlerSuite) TestHandle_NotFound() {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	req := s.newRequestWithID(id.String())
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(nil, usecases.ErrCategoryNotFound)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Contains(rec.Body.String(), "not_found")
}

func (s *GetCategoryHandlerSuite) TestHandle_InvalidID() {
	req := s.newRequestWithID("invalid-uuid")
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Contains(rec.Body.String(), "invalid_query")
}

func (s *GetCategoryHandlerSuite) TestHandle_NotModified() {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	req := s.newRequestWithID(id.String())
	req.Header.Set("If-None-Match", `"v10"`)
	rec := httptest.NewRecorder()

	expectedOutput := &output.CategoryDetailOutput{
		ID:      id,
		Name:    "Test",
		Version: 10,
	}

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusNotModified, rec.Code)
}

func (s *GetCategoryHandlerSuite) TestHandle_IncludeDeprecated() {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	req := s.newRequestWithID(id.String())
	req.URL.RawQuery = "include_deprecated=true"
	rec := httptest.NewRecorder()

	expectedOutput := &output.CategoryDetailOutput{
		ID:      id,
		Name:    "Deprecated Category",
		Version: 1,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.GetCategoryInput) bool {
		return in.IncludeDeprecated == true
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetCategoryHandlerSuite) TestHandle_InternalError() {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	req := s.newRequestWithID(id.String())
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(nil, errors.New("database error"))

	s.handler.Handle(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
}
