package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestParseDirection(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    valueobjects.Direction
		wantErr error
	}{
		{name: "income", input: "income", want: valueobjects.DirectionIncome},
		{name: "outcome", input: "outcome", want: valueobjects.DirectionOutcome},
		{name: "invalid", input: "unknown", wantErr: valueobjects.ErrDirectionUnknown},
		{name: "empty", input: "", wantErr: valueobjects.ErrDirectionUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := valueobjects.ParseDirection(tc.input)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, d)
			assert.Equal(t, tc.input, d.String())
		})
	}
}

func TestDirectionIota(t *testing.T) {
	assert.Equal(t, valueobjects.Direction(1), valueobjects.DirectionIncome)
	assert.Equal(t, valueobjects.Direction(2), valueobjects.DirectionOutcome)
}
