package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
)

type mockListCategoriesUseCase struct {
	mock.Mock
}

func (m *mockListCategoriesUseCase) Execute(ctx context.Context, in *input.ListCategoriesInput) (*output.ListCategoriesOutput, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*output.ListCategoriesOutput), args.Error(1)
}

type ListCategoriesHandlerSuite struct {
	suite.Suite
	handler *ListCategoriesHandler
	mockUC  *mockListCategoriesUseCase
}

func (s *ListCategoriesHandlerSuite) SetupTest() {
	s.mockUC = new(mockListCategoriesUseCase)
	s.handler = NewListCategoriesHandler(s.mockUC, noop.NewProvider())
}

func TestListCategoriesHandlerSuite(t *testing.T) {
	suite.Run(t, new(ListCategoriesHandlerSuite))
}

func (s *ListCategoriesHandlerSuite) TestHandle_Success() {
	req := httptest.NewRequest(http.MethodGet, "/categories?kind=income", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.ListCategoriesOutput{
		Categories: []output.CategoryTreeOutput{
			{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Name: "Salario", Kind: "income", Version: 42},
		},
		Version: 42,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.ListCategoriesInput) bool {
		return in.Kind != nil && in.Kind.String() == "income"
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("\"v42\"", rec.Header().Get("ETag"))

	var response output.ListCategoriesOutput
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal(int64(42), response.Version)
	s.Len(response.Categories, 1)
}

func (s *ListCategoriesHandlerSuite) TestHandle_InvalidKind() {
	req := httptest.NewRequest(http.MethodGet, "/categories?kind=invalid", nil)
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Contains(rec.Body.String(), "invalid_kind")
}

func (s *ListCategoriesHandlerSuite) TestHandle_NotModified() {
	req := httptest.NewRequest(http.MethodGet, "/categories?kind=income", nil)
	req.Header.Set("If-None-Match", "\"v42\"")
	rec := httptest.NewRecorder()

	expectedOutput := &output.ListCategoriesOutput{
		Categories: []output.CategoryTreeOutput{},
		Version:    42,
	}

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusNotModified, rec.Code)
	s.Empty(rec.Body.Bytes())
}

func (s *ListCategoriesHandlerSuite) TestHandle_UseCaseError() {
	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(nil, errors.New("database error"))

	s.handler.Handle(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *ListCategoriesHandlerSuite) TestHandle_WithParentID() {
	parentID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	req := httptest.NewRequest(http.MethodGet, "/categories?parent_id=22222222-2222-2222-2222-222222222222", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.ListCategoriesOutput{
		Categories: []output.CategoryTreeOutput{
			{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), Name: "Subcategoria", Kind: "expense", Version: 1},
		},
		Version: 1,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.ListCategoriesInput) bool {
		return in.ParentID != nil && *in.ParentID == parentID
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListCategoriesHandlerSuite) TestHandle_InvalidParentID() {
	req := httptest.NewRequest(http.MethodGet, "/categories?parent_id=invalid-uuid", nil)
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Contains(rec.Body.String(), "invalid_query")
}

func (s *ListCategoriesHandlerSuite) TestHandle_IncludeDeprecated() {
	req := httptest.NewRequest(http.MethodGet, "/categories?include_deprecated=true", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.ListCategoriesOutput{
		Categories: []output.CategoryTreeOutput{},
		Version:    1,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.ListCategoriesInput) bool {
		return in.IncludeDeprecated == true
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}
