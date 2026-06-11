package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type SearchOutcomeSuite struct {
	suite.Suite
}

func TestSearchOutcomeSuite(t *testing.T) {
	suite.Run(t, new(SearchOutcomeSuite))
}

func (s *SearchOutcomeSuite) TestClassifyOutcome() {
	scenarios := []struct {
		name  string
		count int
		want  valueobjects.SearchOutcome
	}{
		{name: "zero candidatos deve ser no_match", count: 0, want: valueobjects.SearchOutcomeNoMatch},
		{name: "contagem negativa deve ser no_match", count: -1, want: valueobjects.SearchOutcomeNoMatch},
		{name: "um candidato deve ser matched", count: 1, want: valueobjects.SearchOutcomeMatched},
		{name: "dois candidatos devem ser ambiguous", count: 2, want: valueobjects.SearchOutcomeAmbiguous},
		{name: "muitos candidatos devem ser ambiguous", count: 10, want: valueobjects.SearchOutcomeAmbiguous},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, valueobjects.ClassifyOutcome(scenario.count))
		})
	}
}

func (s *SearchOutcomeSuite) TestSearchOutcomeString() {
	scenarios := []struct {
		name    string
		outcome valueobjects.SearchOutcome
		want    string
	}{
		{name: "no_match deve serializar como no_match", outcome: valueobjects.SearchOutcomeNoMatch, want: "no_match"},
		{name: "matched deve serializar como matched", outcome: valueobjects.SearchOutcomeMatched, want: "matched"},
		{name: "ambiguous deve serializar como ambiguous", outcome: valueobjects.SearchOutcomeAmbiguous, want: "ambiguous"},
		{name: "unknown deve serializar como vazio", outcome: valueobjects.SearchOutcomeUnknown, want: ""},
		{name: "valor invalido deve serializar como vazio", outcome: valueobjects.SearchOutcome(99), want: ""},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.outcome.String())
		})
	}
}

func (s *SearchOutcomeSuite) TestSearchOutcomeIsValid() {
	scenarios := []struct {
		name    string
		outcome valueobjects.SearchOutcome
		want    bool
	}{
		{name: "no_match eh valido", outcome: valueobjects.SearchOutcomeNoMatch, want: true},
		{name: "matched eh valido", outcome: valueobjects.SearchOutcomeMatched, want: true},
		{name: "ambiguous eh valido", outcome: valueobjects.SearchOutcomeAmbiguous, want: true},
		{name: "unknown eh invalido", outcome: valueobjects.SearchOutcomeUnknown, want: false},
		{name: "valor desconhecido eh invalido", outcome: valueobjects.SearchOutcome(99), want: false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.outcome.IsValid())
		})
	}
}
