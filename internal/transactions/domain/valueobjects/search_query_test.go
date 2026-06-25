package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestNewSearchQuery(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "valid", input: "uber", want: "uber"},
		{name: "valid trims surrounding spaces", input: "  mercado  ", want: "mercado"},
		{name: "valid exactly two chars", input: "ab", want: "ab"},
		{name: "invalid empty", input: "", wantErr: valueobjects.ErrSearchQueryTooShort},
		{name: "invalid only spaces", input: "   ", wantErr: valueobjects.ErrSearchQueryTooShort},
		{name: "invalid single char", input: "a", wantErr: valueobjects.ErrSearchQueryTooShort},
		{name: "invalid single char after trim", input: "  x  ", wantErr: valueobjects.ErrSearchQueryTooShort},
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
			assert.Equal(t, tc.want, q.String())
		})
	}
}
