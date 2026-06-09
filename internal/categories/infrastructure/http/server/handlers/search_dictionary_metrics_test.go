package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type SearchDictionaryMetricsSuite struct {
	suite.Suite
	handler *SearchDictionaryHandler
	mockUC  *mockSearchDictionaryUseCase
	o11y    *fake.Provider
}

func (s *SearchDictionaryMetricsSuite) SetupTest() {
	s.mockUC = new(mockSearchDictionaryUseCase)
	s.o11y = fake.NewProvider()
	s.handler = NewSearchDictionaryHandler(s.mockUC, s.o11y)
}

func TestSearchDictionaryMetricsSuite(t *testing.T) {
	suite.Run(t, new(SearchDictionaryMetricsSuite))
}

func (s *SearchDictionaryMetricsSuite) counterValuesByLabel(metric, labelKey string) map[string]int64 {
	c := s.o11y.Metrics().(*fake.FakeMetrics).GetCounter(metric)
	if c == nil {
		return nil
	}
	totals := make(map[string]int64)
	for _, v := range c.GetValues() {
		label := ""
		for _, f := range v.Fields {
			if f.Key == labelKey {
				label = f.StringValue()
				break
			}
		}
		totals[label] += v.Value
	}
	return totals
}

func (s *SearchDictionaryMetricsSuite) counterValuesByLabels(metric string, labelFilters map[string]string) int64 {
	c := s.o11y.Metrics().(*fake.FakeMetrics).GetCounter(metric)
	if c == nil {
		return 0
	}
	var total int64
	for _, v := range c.GetValues() {
		match := true
		for key, expectedValue := range labelFilters {
			found := false
			for _, f := range v.Fields {
				if f.Key == key && f.StringValue() == expectedValue {
					found = true
					break
				}
			}
			if !found {
				match = false
				break
			}
		}
		if match {
			total += v.Value
		}
	}
	return total
}

func (s *SearchDictionaryMetricsSuite) logEntries() []fake.LogEntry {
	return s.o11y.Logger().(*fake.FakeLogger).GetEntries()
}

func (s *SearchDictionaryMetricsSuite) TestMetrics_Matched() {
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
		HasMore:       false,
		Version:       42,
		SignalTypeTop: "canonical_name",
		IsAmbiguous:   false,
	}

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	byOutcome := s.counterValuesByLabel("category_dictionary_search_total", "outcome")
	s.Equal(int64(1), byOutcome["matched"])

	byQLen := s.counterValuesByLabel("category_dictionary_search_total", "q_len_bucket")
	s.Equal(int64(1), byQLen["5-8"])

	bySignalType := s.counterValuesByLabel("category_dictionary_search_total", "signal_type_top")
	s.Equal(int64(1), bySignalType["canonical_name"])
}

func (s *SearchDictionaryMetricsSuite) TestMetrics_NoMatch() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=xyz123&kind=expense", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.DictionarySearchOutput{
		Result:  "no_match",
		Version: 42,
	}

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	byOutcome := s.counterValuesByLabel("category_dictionary_search_total", "outcome")
	s.Equal(int64(1), byOutcome["no_match"])
}

func (s *SearchDictionaryMetricsSuite) TestMetrics_Ambiguous() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=uber&kind=expense", nil)
	rec := httptest.NewRecorder()

	expectedOutput := &output.DictionarySearchOutput{
		Result: "candidates",
		Candidates: []output.CandidateOutput{
			{
				CategoryID:  uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				Path:        "Custo Fixo > Transporte por Aplicativo Recorrente",
				MatchedTerm: "uber",
				SignalType:  "merchant",
				Confidence:  "low",
				IsAmbiguous: true,
			},
			{
				CategoryID:  uuid.MustParse("22222222-2222-2222-2222-222222222222"),
				Path:        "Prazeres > Transporte de Lazer",
				MatchedTerm: "uber",
				SignalType:  "merchant",
				Confidence:  "low",
				IsAmbiguous: true,
			},
		},
		HasMore:     false,
		Version:     42,
		IsAmbiguous: true,
	}

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	byOutcome := s.counterValuesByLabel("category_dictionary_search_total", "outcome")
	s.Equal(int64(1), byOutcome["ambiguous"])
}

func (s *SearchDictionaryMetricsSuite) TestMetrics_InvalidKind() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=salario&kind=invalid", nil)
	rec := httptest.NewRecorder()

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)

	byOutcome := s.counterValuesByLabel("category_dictionary_search_total", "outcome")
	s.Equal(int64(1), byOutcome["invalid_kind"])
}

func (s *SearchDictionaryMetricsSuite) TestMetrics_InvalidQuery() {
	req := httptest.NewRequest(http.MethodGet, "/category-dictionary/search?q=ab&kind=income", nil)
	rec := httptest.NewRecorder()

	s.mockUC.On("Execute", mock.Anything, mock.Anything).Return(nil, valueobjects.ErrInvalidQuery)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusUnprocessableEntity, rec.Code)

	byOutcome := s.counterValuesByLabel("category_dictionary_search_total", "outcome")
	s.Equal(int64(1), byOutcome["invalid_query"])

	byQLen := s.counterValuesByLabel("category_dictionary_search_total", "q_len_bucket")
	s.Equal(int64(1), byQLen[""])
}

func (s *SearchDictionaryMetricsSuite) TestQLenBucket_Calculation() {
	tests := []struct {
		query    string
		expected string
	}{
		{"ab", ""},
		{"abc", "3-4"},
		{"abcd", "3-4"},
		{"abcde", "5-8"},
		{"abcdefgh", "5-8"},
		{"abcdefghi", "9-16"},
		{"abcdefghijklmnop", "9-16"},
		{"abcdefghijklmnopq", "17-32"},
		{"abcdefghijklmnopqrstuvwxyz012345", "17-32"},
		{"abcdefghijklmnopqrstuvwxyz0123456", "33+"},
		{"  salario  ", "5-8"},
		{"s@l@r!o", "3-4"},
	}

	for _, tt := range tests {
		s.Run(tt.query, func() {
			result := s.handler.calcQLenBucket(tt.query)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *SearchDictionaryMetricsSuite) TestLogs_DoNotContainRawQuery() {
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
		HasMore:       false,
		Version:       42,
		SignalTypeTop: "canonical_name",
		IsAmbiguous:   false,
	}

	s.mockUC.On("Execute", mock.Anything, mock.MatchedBy(func(in *input.SearchDictionaryInput) bool {
		return in.Query == "salario"
	})).Return(expectedOutput, nil)

	s.handler.Handle(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	entries := s.logEntries()
	s.NotEmpty(entries)

	for _, entry := range entries {
		for _, field := range entry.Fields {
			s.NotEqual("query", field.Key, "log must not contain query field")
			s.NotEqual("q", field.Key, "log must not contain q field")
			s.NotContains(field.StringValue(), "salario", "log must not contain raw query value")
		}
	}
}
