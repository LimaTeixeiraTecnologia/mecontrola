package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestParseFrequency(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    valueobjects.Frequency
		wantErr error
	}{
		{name: "monthly", input: "monthly", want: valueobjects.FrequencyMonthly},
		{name: "yearly", input: "yearly", want: valueobjects.FrequencyYearly},
		{name: "invalid", input: "weekly", wantErr: valueobjects.ErrFrequencyUnknown},
		{name: "empty", input: "", wantErr: valueobjects.ErrFrequencyUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := valueobjects.ParseFrequency(tc.input)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, f)
			assert.Equal(t, tc.input, f.String())
		})
	}
}

func TestFrequencyIota(t *testing.T) {
	assert.Equal(t, valueobjects.Frequency(1), valueobjects.FrequencyMonthly)
	assert.Equal(t, valueobjects.Frequency(2), valueobjects.FrequencyYearly)
}
