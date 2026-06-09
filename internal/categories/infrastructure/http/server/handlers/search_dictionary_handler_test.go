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

type mockSearchDictionaryUseCase struct {
	mock.Mock
}

func (m *mockSearchDictionaryUseCase) Execute(ctx context.Context, in *input.SearchDictionaryInput) (*output.DictionarySearchOutput, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*output.DictionarySearchOutput), args.Error(1)
}

type SearchDictionaryHandlerSuite struct {
	suite.Suite
	handler *SearchDictionaryHandler
	mockUC  *mockSearchDictionaryUseCase
}

func (s *SearchDictionaryHandlerSuite) SetupTest() {
	s.mockUC = new(mockSearchDictionaryUseCase)
	s.handler = NewSearchDictionaryHandler(s.mockUC, noop.NewProvider())
}

func TestSearchDictionaryHandlerSuite(t *testing.T) {
	suite.Run(t, new(SearchDictionaryHandlerSuite))
}

func (s *SearchDictionaryHandlerSuite) TestHandle_Success_Candidates() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=salario&kind=income", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.DictionarySearchOutput{
		Result: "candidates",
		Candidates: []output.CandidateOutput{
			{
				CategoryID:  uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				Path:        "Salario > Salario",
				MatchedTerm: "salario",
				SignalType:  "canonical_name",
				Confidence:  "high",
			},
		},
		HasMore: false,
		Version: 42,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.SearchDictionaryInput) bool {
		return in.Query == "salario" && in.Kind == valueobjects.KindIncome
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal(`"v42"`, rec.Header().Get("ETag"))

	var response output.DictionarySearchOutput
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal("candidates", response.Result)
	s.Len(response.Candidates, 1)
}

func (s *SearchDictionaryHandlerSuite) TestHandle_Success_NoMatch() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=xyz123&kind=expense", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.DictionarySearchOutput{
		Result:  "no_match",
		Version: 42,
	}

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var response output.DictionarySearchOutput
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	s.NoError(err)
	s.Equal("no_match", response.Result)
	s.Empty(response.Candidates)
}

func (s *SearchDictionaryHandlerSuite) TestHandle_MissingKind() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=salario", nil)
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Contains(rec.Body.String(), "invalid_kind")
}

func (s *SearchDictionaryHandlerSuite) TestHandle_InvalidKind() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=salario&kind=invalid", nil)
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Contains(rec.Body.String(), "invalid_kind")
}

func (s *SearchDictionaryHandlerSuite) TestHandle_InvalidQuery() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=ab&kind=income", nil)
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(nil, valueobjects.ErrInvalidQuery)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Contains(rec.Body.String(), "invalid_query")
}

func (s *SearchDictionaryHandlerSuite) TestHandle_NotModified() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=salario&kind=income", nil)
	req.Header.Set("If-None-Match", `"v10"`)
	rec := httptest.NewRecorder()

	expectedOutput := &output.DictionarySearchOutput{
		Result:  "candidates",
		Version: 10,
	}

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusNotModified, rec.Code)
}

func (s *SearchDictionaryHandlerSuite) TestHandle_InternalError() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=salario&kind=income", nil)
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(nil, errors.New("database error"))

	s.handler.Handle(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *SearchDictionaryHandlerSuite) TestHandle_EmptyQuery() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=&kind=expense", nil)
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(nil, valueobjects.ErrInvalidQuery)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.Contains(rec.Body.String(), "invalid_query")
}

func (s *SearchDictionaryHandlerSuite) TestHandle_ETagInErrorResponse() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=ab&kind=income", nil)
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(nil, valueobjects.ErrInvalidQuery)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)
	s.NotEmpty(rec.Header().Get("ETag"))
}
