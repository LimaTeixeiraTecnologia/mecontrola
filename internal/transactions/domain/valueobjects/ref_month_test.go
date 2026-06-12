package valueobjects_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestNewRefMonth(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "valid jan", input: "2026-01"},
		{name: "valid dec", input: "2026-12"},
		{name: "valid mid", input: "2024-06"},
		{name: "invalid short", input: "2026-1", wantErr: valueobjects.ErrRefMonthInvalid},
		{name: "invalid month 00", input: "2026-00", wantErr: valueobjects.ErrRefMonthInvalid},
		{name: "invalid month 13", input: "2026-13", wantErr: valueobjects.ErrRefMonthInvalid},
		{name: "invalid letters", input: "YYYY-MM", wantErr: valueobjects.ErrRefMonthInvalid},
		{name: "invalid empty", input: "", wantErr: valueobjects.ErrRefMonthInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rm, err := valueobjects.NewRefMonth(tc.input)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.input, rm.String())
		})
	}
}

func TestRefMonthFromTime(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)

	ts := time.Date(2026, 6, 15, 10, 0, 0, 0, loc)
	rm := valueobjects.RefMonthFromTime(ts, loc)
	assert.Equal(t, "2026-06", rm.String())
}

func TestRefMonthNext(t *testing.T) {
	rm, _ := valueobjects.NewRefMonth("2026-12")
	next := rm.Next()
	assert.Equal(t, "2027-01", next.String())
}
