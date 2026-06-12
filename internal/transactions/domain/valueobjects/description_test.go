package valueobjects_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestNewDescription(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "valid short", input: "Supermercado"},
		{name: "valid max", input: strings.Repeat("a", 500)},
		{name: "invalid empty", input: "", wantErr: valueobjects.ErrDescriptionEmpty},
		{name: "invalid too long", input: strings.Repeat("a", 501), wantErr: valueobjects.ErrDescriptionTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := valueobjects.NewDescription(tc.input)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.input, d.String())
		})
	}
}
