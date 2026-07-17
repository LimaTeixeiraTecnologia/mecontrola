package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateEntryAmount(t *testing.T) {
	scenarios := []struct {
		name    string
		cents   int64
		wantErr error
		wantNil bool
	}{
		{
			name:    "rejeita zero",
			cents:   0,
			wantErr: errAmountNonPositive,
		},
		{
			name:    "rejeita negativo",
			cents:   -1,
			wantErr: errAmountNonPositive,
		},
		{
			name:    "aceita valor normal",
			cents:   15000,
			wantNil: true,
		},
		{
			name:    "aceita no teto",
			cents:   maxEntryAmountCents,
			wantNil: true,
		},
		{
			name:    "rejeita acima do teto",
			cents:   maxEntryAmountCents + 1,
			wantErr: errAmountAboveCeiling,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			err := validateEntryAmount(s.cents)
			if s.wantNil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, s.wantErr)
		})
	}
}

func TestValidateEntryDescription(t *testing.T) {
	scenarios := []struct {
		name        string
		description string
		wantErr     bool
	}{
		{name: "rejeita vazio", description: "", wantErr: true},
		{name: "rejeita apenas espacos", description: "   ", wantErr: true},
		{name: "aceita termo literal", description: "mercado", wantErr: false},
		{name: "aceita com espacos ao redor", description: "  padaria  ", wantErr: false},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			err := validateEntryDescription(s.description)
			if s.wantErr {
				require.ErrorIs(t, err, errDescriptionEmpty)
				return
			}
			require.NoError(t, err)
		})
	}
}
