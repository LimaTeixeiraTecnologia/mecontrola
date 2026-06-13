package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func TestNewClientIP(t *testing.T) {
	scenarios := []struct {
		name       string
		input      string
		wantErr    bool
		wantZero   bool
		wantString string
	}{
		{
			name:     "vazio retorna IsZero sem erro",
			input:    "",
			wantErr:  false,
			wantZero: true,
		},
		{
			name:       "IP simples valido",
			input:      "1.2.3.4",
			wantErr:    false,
			wantZero:   false,
			wantString: "1.2.3.4",
		},
		{
			name:       "lista com dois IPs retorna ultimo",
			input:      "evil, 1.2.3.4",
			wantErr:    false,
			wantZero:   false,
			wantString: "1.2.3.4",
		},
		{
			name:    "IP invalido retorna erro",
			input:   "evil",
			wantErr: true,
		},
		{
			name:       "lista com IPv4 e IPv6 retorna ultimo (IPv6)",
			input:      "1.2.3.4, ::1",
			wantErr:    false,
			wantZero:   false,
			wantString: "::1",
		},
		{
			name:       "lista com tres IPs retorna ultimo",
			input:      "1.2.3.4,1.2.3.5,1.2.3.6",
			wantErr:    false,
			wantZero:   false,
			wantString: "1.2.3.6",
		},
		{
			name:     "apenas espacos retorna IsZero sem erro",
			input:    "   ",
			wantErr:  false,
			wantZero: true,
		},
		{
			name:     "lista terminando em virgula com ultimo elemento vazio retorna IsZero sem erro",
			input:    "1.2.3.4,",
			wantErr:  false,
			wantZero: true,
		},
		{
			name:       "IPv6 valido standalone",
			input:      "2001:db8::1",
			wantErr:    false,
			wantZero:   false,
			wantString: "2001:db8::1",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			ip, err := valueobjects.NewClientIP(sc.input)
			if sc.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, valueobjects.ErrClientIPInvalid))
				assert.True(t, ip.IsZero())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, sc.wantZero, ip.IsZero())
			if !sc.wantZero {
				assert.Equal(t, sc.wantString, ip.String())
			}
		})
	}
}

func TestClientIPIsZero(t *testing.T) {
	var zero valueobjects.ClientIP
	assert.True(t, zero.IsZero())
	assert.Equal(t, "", zero.String())
}
