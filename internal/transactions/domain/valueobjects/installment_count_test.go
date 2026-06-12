package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestNewInstallmentCount(t *testing.T) {
	cases := []struct {
		name    string
		n       int
		wantErr error
	}{
		{name: "valid min", n: 1},
		{name: "valid max", n: 24},
		{name: "valid mid", n: 12},
		{name: "invalid zero", n: 0, wantErr: valueobjects.ErrInstallmentCountOutOfRange},
		{name: "invalid 25", n: 25, wantErr: valueobjects.ErrInstallmentCountOutOfRange},
		{name: "invalid negative", n: -1, wantErr: valueobjects.ErrInstallmentCountOutOfRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ic, err := valueobjects.NewInstallmentCount(tc.n)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.n, ic.Value())
		})
	}
}
