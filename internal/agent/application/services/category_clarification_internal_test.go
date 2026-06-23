package services

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CategoryClarificationSuite struct {
	suite.Suite
}

func TestCategoryClarificationSuite(t *testing.T) {
	suite.Run(t, new(CategoryClarificationSuite))
}

func (s *CategoryClarificationSuite) TestFormatCategoryNeedsConfirmation() {
	cases := []struct {
		name       string
		candidates []string
		wantSubstr string
	}{
		{name: "usa o primeiro candidato", candidates: []string{"Prazeres > Streaming", "Custo Fixo"}, wantSubstr: "Prazeres > Streaming"},
		{name: "ignora vazios e usa o proximo", candidates: []string{"  ", "Conhecimento"}, wantSubstr: "Conhecimento"},
		{name: "sem candidatos pede a categoria", candidates: nil, wantSubstr: "Me diz qual categoria"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Contains(formatCategoryNeedsConfirmation(tc.candidates), tc.wantSubstr)
		})
	}
}
