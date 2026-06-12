package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestNewMoney(t *testing.T) {
	cases := []struct {
		name    string
		cents   int64
		wantErr error
	}{
		{name: "valid positive", cents: 100},
		{name: "valid one cent", cents: 1},
		{name: "invalid zero", cents: 0, wantErr: valueobjects.ErrMoneyMustBePositive},
		{name: "invalid negative", cents: -50, wantErr: valueobjects.ErrMoneyMustBePositive},
		{name: "invalid large negative", cents: -99999, wantErr: valueobjects.ErrMoneyMustBePositive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := valueobjects.NewMoney(tc.cents)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.cents, m.Cents())
		})
	}
}
