package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type RootSlugSuite struct {
	suite.Suite
}

func TestRootSlugSuite(t *testing.T) {
	suite.Run(t, new(RootSlugSuite))
}

func (s *RootSlugSuite) TestParseRootSlug() {
	type testCase struct {
		name    string
		input   string
		want    valueobjects.RootSlug
		wantErr bool
	}

	cases := []testCase{
		{name: "custo_fixo", input: "expense.custo_fixo", want: valueobjects.RootSlugCustoFixo, wantErr: false},
		{name: "conhecimento", input: "expense.conhecimento", want: valueobjects.RootSlugConhecimento, wantErr: false},
		{name: "prazeres", input: "expense.prazeres", want: valueobjects.RootSlugPrazeres, wantErr: false},
		{name: "metas", input: "expense.metas", want: valueobjects.RootSlugMetas, wantErr: false},
		{name: "liberdade_financeira", input: "expense.liberdade_financeira", want: valueobjects.RootSlugLiberdadeFinanceira, wantErr: false},
		{name: "desconhecido", input: "expense.outro", wantErr: true},
		{name: "vazio", input: "", wantErr: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.ParseRootSlug(tc.input)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tc.want, got)
			s.Equal(tc.input, got.String())
		})
	}
}

func (s *RootSlugSuite) TestCanonicalOrder() {
	order := valueobjects.CanonicalOrder()
	s.Equal(valueobjects.RootSlugCustoFixo, order[0])
	s.Equal(valueobjects.RootSlugConhecimento, order[1])
	s.Equal(valueobjects.RootSlugPrazeres, order[2])
	s.Equal(valueobjects.RootSlugMetas, order[3])
	s.Equal(valueobjects.RootSlugLiberdadeFinanceira, order[4])
}
