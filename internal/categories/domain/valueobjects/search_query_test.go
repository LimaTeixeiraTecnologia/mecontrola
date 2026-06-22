package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

func TestNewSearchQuery(t *testing.T) {
	cases := []struct {
		name           string
		input          string
		wantNormalized string
		wantTrimmed    string
		wantErr        error
	}{
		{name: "valid simple", input: "aluguel", wantNormalized: "aluguel", wantTrimmed: "aluguel"},
		{name: "valid uppercase", input: "ALUGUEL", wantNormalized: "aluguel", wantTrimmed: "ALUGUEL"},
		{name: "valid trims spaces", input: "  energia  ", wantNormalized: "energia", wantTrimmed: "energia"},
		{name: "valid strips punctuation", input: "  ALUGUEL!!!  ", wantNormalized: "aluguel", wantTrimmed: "ALUGUEL!!!"},
		{name: "valid keeps digits", input: "plano2026", wantNormalized: "plano2026", wantTrimmed: "plano2026"},
		{name: "invalid empty", input: "", wantErr: valueobjects.ErrInvalidQuery},
		{name: "invalid only spaces", input: "   ", wantErr: valueobjects.ErrInvalidQuery},
		{name: "invalid only punctuation", input: "...!!!", wantErr: valueobjects.ErrInvalidQuery},
		{name: "invalid too short", input: "ab", wantErr: valueobjects.ErrInvalidQuery},
		{name: "invalid two letters with punctuation", input: "a!b", wantErr: valueobjects.ErrInvalidQuery},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := valueobjects.NewSearchQuery(tc.input)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantNormalized, q.Normalized())
			assert.Equal(t, tc.wantTrimmed, q.Trimmed())
			assert.Equal(t, tc.input, q.Raw())
		})
	}
}

func TestNormalizeSearchQuery(t *testing.T) {
	assert.Equal(t, "aluguel", valueobjects.NormalizeSearchQuery("  ALUGUEL!!!  "))
	assert.Equal(t, "", valueobjects.NormalizeSearchQuery("   "))
	assert.Equal(t, "abc123", valueobjects.NormalizeSearchQuery("abc-123"))
}

func TestSearchQueryTokens(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "remove verbo e adverbio stopword", input: "paguei netflix hoje", want: []string{"netflix"}},
		{name: "preserva fronteira multi-token", input: "transporte por aplicativo", want: []string{"transporte", "aplicativo"}},
		{name: "remove preposicoes", input: "conta de energia da casa", want: []string{"conta", "energia", "casa"}},
		{name: "dedup tokens repetidos", input: "uber uber uber", want: []string{"uber"}},
		{name: "lowercase e pontuacao", input: "IPTU, 2026!", want: []string{"iptu", "2026"}},
		{name: "remove acentos dos tokens", input: "saúde mental", want: []string{"saude", "mental"}},
		{name: "stopword acentuada e removida", input: "compra para amanhã", want: []string{"compra"}},
		{name: "token unico", input: "mercado", want: []string{"mercado"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := valueobjects.NewSearchQuery(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, q.Tokens())
		})
	}
}
