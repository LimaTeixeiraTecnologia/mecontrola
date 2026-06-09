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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type mockListDictionaryUseCase struct {
	mock.Mock
}

func (m *mockListDictionaryUseCase) Execute(ctx context.Context, in *input.ListDictionaryInput) (*output.ListDictionaryOutput, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*output.ListDictionaryOutput), args.Error(1)
}

type ListDictionaryHandlerSuite struct {
	suite.Suite
	handler *ListDictionaryHandler
	mockUC  *mockListDictionaryUseCase
}

func (s *ListDictionaryHandlerSuite) SetupTest() {
	s.mockUC = new(mockListDictionaryUseCase)
	s.handler = NewListDictionaryHandler(s.mockUC, noop.NewProvider())
}

func TestListDictionaryHandlerSuite(t *testing.T) {
	suite.Run(t, new(ListDictionaryHandlerSuite))
}

func (s *ListDictionaryHandlerSuite) TestHandle_Success() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary?kind=income", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.ListDictionaryOutput{
		Entries: []output.DictionaryEntryOutput{
			{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Term: "salario", Kind: "income"},
		},
		Version: 42,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.ListDictionaryInput) bool {
		return in.Kind != nil && in.Kind.String() == "income"
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal(`"v42"`, rec.Header().Get("ETag"))

	var response output.ListDictionaryOutput
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.NoError(err)
	s.Len(response.Entries, 1)
}

func (s *ListDictionaryHandlerSuite) TestHandle_InvalidKind() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary?kind=invalid", nil)
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Contains(rec.Body.String(), "invalid_kind")
}

func (s *ListDictionaryHandlerSuite) TestHandle_InvalidSignalType() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary?signal_type=invalid", nil)
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Contains(rec.Body.String(), "invalid_query")
}

func (s *ListDictionaryHandlerSuite) TestHandle_WithCategoryID() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary?category_id=11111111-1111-1111-1111-111111111111", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.ListDictionaryOutput{
		Entries: []output.DictionaryEntryOutput{},
		Version: 1,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.ListDictionaryInput) bool {
		return in.CategoryID != nil && *in.CategoryID == "11111111-1111-1111-1111-111111111111"
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListDictionaryHandlerSuite) TestHandle_WithCursor() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary?cursor=abc123", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.ListDictionaryOutput{
		Entries:    []output.DictionaryEntryOutput{},
		NextCursor: "next_cursor_123",
		Version:    1,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.ListDictionaryInput) bool {
		return in.Cursor == "abc123"
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var response output.ListDictionaryOutput
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal("next_cursor_123", response.NextCursor)
}

func (s *ListDictionaryHandlerSuite) TestHandle_WithPageSize() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary?page_size=100", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.ListDictionaryOutput{
		Entries: []output.DictionaryEntryOutput{},
		Version: 1,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.ListDictionaryInput) bool {
		return in.PageSize == 100
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListDictionaryHandlerSuite) TestHandle_NotModified() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary", nil)
	req.Header.Set("If-None-Match", `"v5"`)
	rec := httptest.NewRecorder()

	expectedOutput := &output.ListDictionaryOutput{
		Entries: []output.DictionaryEntryOutput{},
		Version: 5,
	}

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusNotModified, rec.Code)
}

func (s *ListDictionaryHandlerSuite) TestHandle_UseCaseError() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary", nil)
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(nil, errors.New("database error"))

	s.handler.Handle(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *ListDictionaryHandlerSuite) TestHandle_WithValidSignalType() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary?signal_type=alias", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.ListDictionaryOutput{
		Entries: []output.DictionaryEntryOutput{},
		Version: 1,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.ListDictionaryInput) bool {
		return in.SignalType != nil && *in.SignalType == valueobjects.SignalTypeAlias
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}
