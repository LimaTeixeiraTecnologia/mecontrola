package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type KindSuite struct {
	suite.Suite
}

func TestKindSuite(t *testing.T) {
	suite.Run(t, new(KindSuite))
}

func (s *KindSuite) TestParseKind() {
	scenarios := []struct {
		name    string
		input   string
		want    valueobjects.Kind
		wantErr bool
	}{
		{name: "deve parsear income", input: "income", want: valueobjects.KindIncome, wantErr: false},
		{name: "deve parsear expense", input: "expense", want: valueobjects.KindExpense, wantErr: false},
		{name: "deve retornar erro para kind invalido", input: "invalid", want: 0, wantErr: true},
		{name: "deve retornar erro para string vazia", input: "", want: 0, wantErr: true},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			got, err := valueobjects.ParseKind(scenario.input)
			if scenario.wantErr {
				s.Error(err)
				s.ErrorIs(err, valueobjects.ErrInvalidKind)
			} else {
				s.NoError(err)
				s.Equal(scenario.want, got)
			}
		})
	}
}

func (s *KindSuite) TestKindString() {
	scenarios := []struct {
		name string
		kind valueobjects.Kind
		want string
	}{
		{name: "income deve retornar income", kind: valueobjects.KindIncome, want: "income"},
		{name: "expense deve retornar expense", kind: valueobjects.KindExpense, want: "expense"},
		{name: "zero value deve retornar vazio", kind: 0, want: ""},
		{name: "valor invalido deve retornar vazio", kind: 99, want: ""},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.kind.String())
		})
	}
}

func (s *KindSuite) TestKindIsValid() {
	scenarios := []struct {
		name string
		kind valueobjects.Kind
		want bool
	}{
		{name: "income eh valido", kind: valueobjects.KindIncome, want: true},
		{name: "expense eh valido", kind: valueobjects.KindExpense, want: true},
		{name: "zero value eh invalido", kind: 0, want: false},
		{name: "valor invalido eh invalido", kind: 99, want: false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, scenario.kind.IsValid())
		})
	}
}
