package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type SignalTypeSuite struct {
	suite.Suite
}

func TestSignalTypeSuite(t *testing.T) {
	suite.Run(t, new(SignalTypeSuite))
}

func (s *SignalTypeSuite) TestParseSignalType() {
	scenarios := []struct {
		name    string
		input   string
		want    valueobjects.SignalType
		wantErr bool
	}{
		{name: "deve parsear canonical_name", input: "canonical_name", want: valueobjects.SignalTypeCanonicalName, wantErr: false},
		{name: "deve parsear alias", input: "alias", want: valueobjects.SignalTypeAlias, wantErr: false},
		{name: "deve parsear phrase", input: "phrase", want: valueobjects.SignalTypePhrase, wantErr: false},
		{name: "deve parsear merchant", input: "merchant", want: valueobjects.SignalTypeMerchant, wantErr: false},
		{name: "deve parsear segment", input: "segment", want: valueobjects.SignalTypeSegment, wantErr: false},
		{name: "deve retornar erro para signal type invalido", input: "invalid", want: 0, wantErr: true},
		{name: "deve retornar erro para string vazia", input: "", want: 0, wantErr: true},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			got, err := valueobjects.ParseSignalType(scenario.input)
			if scenario.wantErr {
				s.Error(err)
				s.ErrorIs(err, valueobjects.ErrInvalidSignalType)
			} else {
				s.NoError(err)
				s.Equal(scenario.want, got)
			}
		})
	}
}

func (s *SignalTypeSuite) TestSignalTypeString() {
	scenarios := []struct {
		name       string
		signalType valueobjects.SignalType
		want       string
	}{
		{name: "canonical_name deve retornar canonical_name", signalType: valueobjects.SignalTypeCanonicalName, want: "canonical_name"},
		{name: "alias deve retornar alias", signalType: valueobjects.SignalTypeAlias, want: "alias"},
		{name: "phrase deve retornar phrase", signalType: valueobjects.SignalTypePhrase, want: "phrase"},
		{name: "merchant deve retornar merchant", signalType: valueobjects.SignalTypeMerchant, want: "merchant"},
		{name: "segment deve retornar segment", signalType: valueobjects.SignalTypeSegment, want: "segment"},
		{name: "zero value deve retornar vazio", signalType: 0, want: ""},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.signalType.String())
		})
	}
}

func (s *SignalTypeSuite) TestSignalTypePrecedence() {
	scenarios := []struct {
		name       string
		signalType valueobjects.SignalType
		want       int
	}{
		{name: "canonical_name tem precedencia 5", signalType: valueobjects.SignalTypeCanonicalName, want: 5},
		{name: "alias tem precedencia 4", signalType: valueobjects.SignalTypeAlias, want: 4},
		{name: "phrase tem precedencia 3", signalType: valueobjects.SignalTypePhrase, want: 3},
		{name: "merchant tem precedencia 2", signalType: valueobjects.SignalTypeMerchant, want: 2},
		{name: "segment tem precedencia 1", signalType: valueobjects.SignalTypeSegment, want: 1},
		{name: "zero value tem precedencia 0", signalType: 0, want: 0},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.signalType.Precedence())
		})
	}
}

func (s *SignalTypeSuite) TestSignalTypeIsValid() {
	scenarios := []struct {
		name       string
		signalType valueobjects.SignalType
		want       bool
	}{
		{name: "canonical_name eh valido", signalType: valueobjects.SignalTypeCanonicalName, want: true},
		{name: "alias eh valido", signalType: valueobjects.SignalTypeAlias, want: true},
		{name: "phrase eh valido", signalType: valueobjects.SignalTypePhrase, want: true},
		{name: "merchant eh valido", signalType: valueobjects.SignalTypeMerchant, want: true},
		{name: "segment eh valido", signalType: valueobjects.SignalTypeSegment, want: true},
		{name: "zero value eh invalido", signalType: 0, want: false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.signalType.IsValid())
		})
	}
}
