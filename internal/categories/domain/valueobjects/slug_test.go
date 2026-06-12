package valueobjects_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

func TestNewSlug(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "valid simple", input: "aluguel"},
		{name: "valid kebab", input: "liberdade-financeira"},
		{name: "valid with digits", input: "plano-2026"},
		{name: "valid min length", input: "ab"},
		{name: "valid max length", input: strings.Repeat("a", 64)},
		{name: "invalid empty", input: "", wantErr: valueobjects.ErrSlugEmpty},
		{name: "invalid single char", input: "a", wantErr: valueobjects.ErrSlugTooShort},
		{name: "invalid too long", input: strings.Repeat("a", 65), wantErr: valueobjects.ErrSlugTooLong},
		{name: "invalid uppercase", input: "Aluguel", wantErr: valueobjects.ErrSlugInvalidChars},
		{name: "invalid accent", input: "informática", wantErr: valueobjects.ErrSlugInvalidChars},
		{name: "invalid underscore", input: "custo_fixo", wantErr: valueobjects.ErrSlugInvalidChars},
		{name: "invalid space", input: "custo fixo", wantErr: valueobjects.ErrSlugInvalidChars},
		{name: "invalid leading hyphen", input: "-aluguel", wantErr: valueobjects.ErrSlugEdgeHyphen},
		{name: "invalid trailing hyphen", input: "aluguel-", wantErr: valueobjects.ErrSlugEdgeHyphen},
		{name: "invalid double hyphen", input: "aluguel--mensal", wantErr: valueobjects.ErrSlugDoubleHyphen},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := valueobjects.NewSlug(tc.input)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr), "expected %v, got %v", tc.wantErr, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.input, s.String())
		})
	}
}

func TestSlugEqual(t *testing.T) {
	a, err := valueobjects.NewSlug("aluguel")
	require.NoError(t, err)
	b, err := valueobjects.NewSlug("aluguel")
	require.NoError(t, err)
	c, err := valueobjects.NewSlug("supermercado")
	require.NoError(t, err)

	assert.True(t, a.Equal(b))
	assert.False(t, a.Equal(c))
}
