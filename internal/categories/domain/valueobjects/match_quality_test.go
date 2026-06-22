package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type MatchQualitySuite struct {
	suite.Suite
}

func TestMatchQualitySuite(t *testing.T) {
	suite.Run(t, new(MatchQualitySuite))
}

func (s *MatchQualitySuite) TestParseMatchQuality() {
	scenarios := []struct {
		name    string
		input   string
		want    valueobjects.MatchQuality
		wantErr bool
	}{
		{name: "deve parsear exact", input: "exact", want: valueobjects.MatchQualityExact},
		{name: "deve parsear token", input: "token", want: valueobjects.MatchQualityToken},
		{name: "deve parsear fuzzy", input: "fuzzy", want: valueobjects.MatchQualityFuzzy},
		{name: "deve retornar erro para invalido", input: "semantic", wantErr: true},
		{name: "deve retornar erro para vazio", input: "", wantErr: true},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			got, err := valueobjects.ParseMatchQuality(scenario.input)
			if scenario.wantErr {
				s.Error(err)
				s.ErrorIs(err, valueobjects.ErrInvalidMatchQuality)
			} else {
				s.NoError(err)
				s.Equal(scenario.want, got)
			}
		})
	}
}

func (s *MatchQualitySuite) TestString() {
	s.Equal("exact", valueobjects.MatchQualityExact.String())
	s.Equal("token", valueobjects.MatchQualityToken.String())
	s.Equal("fuzzy", valueobjects.MatchQualityFuzzy.String())
	s.Equal("", valueobjects.MatchQuality(0).String())
}

func (s *MatchQualitySuite) TestIsValid() {
	s.True(valueobjects.MatchQualityExact.IsValid())
	s.True(valueobjects.MatchQualityToken.IsValid())
	s.True(valueobjects.MatchQualityFuzzy.IsValid())
	s.False(valueobjects.MatchQuality(0).IsValid())
	s.False(valueobjects.MatchQuality(9).IsValid())
}

func (s *MatchQualitySuite) TestWeightIsMonotonic() {
	s.Greater(valueobjects.MatchQualityExact.Weight(), valueobjects.MatchQualityToken.Weight())
	s.Greater(valueobjects.MatchQualityToken.Weight(), valueobjects.MatchQualityFuzzy.Weight())
	s.Equal(0.0, valueobjects.MatchQuality(0).Weight())
}
