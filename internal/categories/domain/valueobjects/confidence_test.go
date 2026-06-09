package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type ConfidenceSuite struct {
	suite.Suite
}

func TestConfidenceSuite(t *testing.T) {
	suite.Run(t, new(ConfidenceSuite))
}

func (s *ConfidenceSuite) TestParseConfidence() {
	scenarios := []struct {
		name    string
		input   string
		want    valueobjects.Confidence
		wantErr bool
	}{
		{name: "deve parsear high", input: "high", want: valueobjects.ConfidenceHigh, wantErr: false},
		{name: "deve parsear medium", input: "medium", want: valueobjects.ConfidenceMedium, wantErr: false},
		{name: "deve parsear low", input: "low", want: valueobjects.ConfidenceLow, wantErr: false},
		{name: "deve retornar erro para confidence invalida", input: "invalid", want: 0, wantErr: true},
		{name: "deve retornar erro para string vazia", input: "", want: 0, wantErr: true},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			got, err := valueobjects.ParseConfidence(scenario.input)
			if scenario.wantErr {
				s.Error(err)
				s.ErrorIs(err, valueobjects.ErrInvalidConfidence)
			} else {
				s.NoError(err)
				s.Equal(scenario.want, got)
			}
		})
	}
}

func (s *ConfidenceSuite) TestConfidenceString() {
	scenarios := []struct {
		name       string
		confidence valueobjects.Confidence
		want       string
	}{
		{name: "high deve retornar high", confidence: valueobjects.ConfidenceHigh, want: "high"},
		{name: "medium deve retornar medium", confidence: valueobjects.ConfidenceMedium, want: "medium"},
		{name: "low deve retornar low", confidence: valueobjects.ConfidenceLow, want: "low"},
		{name: "zero value deve retornar vazio", confidence: 0, want: ""},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.confidence.String())
		})
	}
}

func (s *ConfidenceSuite) TestConfidenceIsValid() {
	scenarios := []struct {
		name       string
		confidence valueobjects.Confidence
		want       bool
	}{
		{name: "high eh valida", confidence: valueobjects.ConfidenceHigh, want: true},
		{name: "medium eh valida", confidence: valueobjects.ConfidenceMedium, want: true},
		{name: "low eh valida", confidence: valueobjects.ConfidenceLow, want: true},
		{name: "zero value eh invalida", confidence: 0, want: false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.confidence.IsValid())
		})
	}
}
