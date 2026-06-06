package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

func TestNewFunnelToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr error
	}{
		{
			name: "accepts valid token",
			raw:  "token-123",
			want: "token-123",
		},
		{
			name: "trims spaces",
			raw:  "  token-123  ",
			want: "token-123",
		},
		{
			name:    "rejects empty token",
			raw:     "",
			wantErr: valueobjects.ErrFunnelTokenEmpty,
		},
		{
			name:    "rejects blank token",
			raw:     "   ",
			wantErr: valueobjects.ErrFunnelTokenEmpty,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			token, err := valueobjects.NewFunnelToken(tt.raw)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, token.String())
		})
	}
}
