package valueobjects_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func TestNewGatewaySignature(t *testing.T) {
	validHex64 := strings.Repeat("a1", 32)

	scenarios := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "aceita 64 chars hex valido lowercase",
			input:   validHex64,
			wantErr: false,
		},
		{
			name:    "rejeita hex com letras maiusculas",
			input:   strings.ToUpper(validHex64),
			wantErr: true,
		},
		{
			name:    "rejeita 63 chars",
			input:   validHex64[:63],
			wantErr: true,
		},
		{
			name:    "rejeita 65 chars",
			input:   validHex64 + "a",
			wantErr: true,
		},
		{
			name:    "rejeita charset invalido",
			input:   strings.Repeat("zz", 32),
			wantErr: true,
		},
		{
			name:    "rejeita string vazia",
			input:   "",
			wantErr: true,
		},
		{
			name:    "rejeita hex com g",
			input:   strings.Repeat("g1", 32),
			wantErr: true,
		},
		{
			name:    "aceita todos os digitos numericos e letras a-f",
			input:   "0123456789abcdef" + strings.Repeat("a1", 24),
			wantErr: false,
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			sig, err := valueobjects.NewGatewaySignature(sc.input)
			if sc.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, valueobjects.ErrGatewaySignatureInvalid))
				assert.True(t, sig.IsZero())
				return
			}
			assert.NoError(t, err)
			assert.False(t, sig.IsZero())
			assert.Len(t, sig.Bytes(), 32)
		})
	}
}

func TestGatewaySignatureBytes(t *testing.T) {
	input := strings.Repeat("ab", 32)
	sig, err := valueobjects.NewGatewaySignature(input)
	assert.NoError(t, err)

	b1 := sig.Bytes()
	b2 := sig.Bytes()
	assert.Equal(t, b1, b2)
	assert.Equal(t, 32, len(b1))

	b1[0] = 0x00
	b3 := sig.Bytes()
	assert.NotEqual(t, b1[0], b3[0], "Bytes deve retornar copia independente")
}

func TestGatewaySignatureIsZero(t *testing.T) {
	var zero valueobjects.GatewaySignature
	assert.True(t, zero.IsZero())
}
