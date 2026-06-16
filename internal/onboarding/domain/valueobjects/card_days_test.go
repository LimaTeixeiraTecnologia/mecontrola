package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

func TestNewCardClosingDay(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		day     int
		wantErr error
	}{
		{"min", 1, nil},
		{"mid", 15, nil},
		{"max", 31, nil},
		{"zero", 0, valueobjects.ErrCardClosingDayOutOfRange},
		{"negative", -1, valueobjects.ErrCardClosingDayOutOfRange},
		{"too_high", 32, valueobjects.ErrCardClosingDayOutOfRange},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := valueobjects.NewCardClosingDay(tc.day)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.day, got.Value())
		})
	}
}

func TestNewCardDueDay(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		day     int
		wantErr error
	}{
		{"min", 1, nil},
		{"mid", 10, nil},
		{"max", 31, nil},
		{"zero", 0, valueobjects.ErrCardDueDayOutOfRange},
		{"too_high", 32, valueobjects.ErrCardDueDayOutOfRange},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := valueobjects.NewCardDueDay(tc.day)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.day, got.Value())
		})
	}
}
