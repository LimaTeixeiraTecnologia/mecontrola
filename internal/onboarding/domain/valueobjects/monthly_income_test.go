package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

func TestNewMonthlyIncome(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		cents   int64
		wantErr error
	}{
		{"minimum_accepted", 50000, nil},
		{"typical", 350000, nil},
		{"maximum_accepted", 10000000000, nil},
		{"below_minimum", 49999, valueobjects.ErrMonthlyIncomeBelowMinimum},
		{"zero", 0, valueobjects.ErrMonthlyIncomeBelowMinimum},
		{"negative", -1, valueobjects.ErrMonthlyIncomeBelowMinimum},
		{"above_maximum", 10000000001, valueobjects.ErrMonthlyIncomeAboveMaximum},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := valueobjects.NewMonthlyIncome(tc.cents)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.cents, got.Cents())
		})
	}
}

func TestMonthlyIncome_Equal(t *testing.T) {
	t.Parallel()
	a, err := valueobjects.NewMonthlyIncome(350000)
	require.NoError(t, err)
	b, err := valueobjects.NewMonthlyIncome(350000)
	require.NoError(t, err)
	c, err := valueobjects.NewMonthlyIncome(400000)
	require.NoError(t, err)
	require.True(t, a.Equal(b))
	require.False(t, a.Equal(c))
}
