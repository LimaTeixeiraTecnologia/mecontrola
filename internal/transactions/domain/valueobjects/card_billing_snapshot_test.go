package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestNewCardBillingSnapshot(t *testing.T) {
	cases := []struct {
		name    string
		closing int
		due     int
		wantErr error
	}{
		{name: "valid typical", closing: 10, due: 17},
		{name: "valid min", closing: 1, due: 1},
		{name: "valid max", closing: 31, due: 31},
		{name: "invalid closing zero", closing: 0, due: 17, wantErr: valueobjects.ErrDayOfMonthOutOfRange},
		{name: "invalid due 32", closing: 10, due: 32, wantErr: valueobjects.ErrDayOfMonthOutOfRange},
		{name: "both invalid", closing: 0, due: 32, wantErr: valueobjects.ErrDayOfMonthOutOfRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap, err := valueobjects.NewCardBillingSnapshot(tc.closing, tc.due)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.closing, snap.ClosingDay().Value())
			assert.Equal(t, tc.due, snap.DueDay().Value())
		})
	}
}
