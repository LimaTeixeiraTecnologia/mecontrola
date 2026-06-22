package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatCategoryNeedsConfirmation(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			assert.Contains(t, formatCategoryNeedsConfirmation(tc.candidates), tc.wantSubstr)
		})
	}
}
