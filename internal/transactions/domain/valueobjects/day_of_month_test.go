package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestNewDayOfMonth(t *testing.T) {
	cases := []struct {
		name    string
		day     int
		wantErr error
	}{
		{name: "valid min", day: 1},
		{name: "valid max", day: 28},
		{name: "valid mid", day: 15},
		{name: "invalid zero", day: 0, wantErr: valueobjects.ErrDayOfMonthOutOfRange},
		{name: "invalid 29", day: 29, wantErr: valueobjects.ErrDayOfMonthOutOfRange},
		{name: "invalid negative", day: -1, wantErr: valueobjects.ErrDayOfMonthOutOfRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := valueobjects.NewDayOfMonth(tc.day)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.day, d.Value())
		})
	}
}

func TestNewDayOfMonthSnapshot(t *testing.T) {
	cases := []struct {
		name    string
		day     int
		wantErr error
	}{
		{name: "valid min", day: 1},
		{name: "valid max", day: 31},
		{name: "valid 28", day: 28},
		{name: "invalid zero", day: 0, wantErr: valueobjects.ErrDayOfMonthOutOfRange},
		{name: "invalid 32", day: 32, wantErr: valueobjects.ErrDayOfMonthOutOfRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := valueobjects.NewDayOfMonthSnapshot(tc.day)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.day, d.Value())
		})
	}
}
