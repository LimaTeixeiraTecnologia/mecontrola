package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func TestNewEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantAddr    string
		wantErr     error
		wantErrWrap bool
	}{
		{
			name:     "email válido",
			input:    "joao@example.com",
			wantAddr: "joao@example.com",
		},
		{
			name:     "email com uppercase normaliza lowercase",
			input:    "JOAO@EXAMPLE.COM",
			wantAddr: "joao@example.com",
		},
		{
			name:     "email com espaços externos",
			input:    "  joao@example.com  ",
			wantAddr: "joao@example.com",
		},
		{
			name:     "subdomínio",
			input:    "user@mail.example.co.uk",
			wantAddr: "user@mail.example.co.uk",
		},
		{
			name:    "vazio",
			input:   "",
			wantErr: valueobjects.ErrEmailInvalid,
		},
		{
			name:    "apenas espaços",
			input:   "   ",
			wantErr: valueobjects.ErrEmailInvalid,
		},
		{
			name:        "sem @",
			input:       "joaosemarroba",
			wantErrWrap: true,
			wantErr:     valueobjects.ErrEmailInvalid,
		},
		{
			name:        "sem domínio",
			input:       "joao@",
			wantErrWrap: true,
			wantErr:     valueobjects.ErrEmailInvalid,
		},
		{
			name:        "formato inválido",
			input:       "@@example.com",
			wantErrWrap: true,
			wantErr:     valueobjects.ErrEmailInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := valueobjects.NewEmail(tc.input)

			if tc.wantErr != nil {
				require.Error(t, err)
				if tc.wantErrWrap {
					assert.True(t, errors.Is(err, tc.wantErr), "erro deve envolver %v, got %v", tc.wantErr, err)
				} else {
					assert.ErrorIs(t, err, tc.wantErr)
				}
				assert.Equal(t, valueobjects.Email{}, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantAddr, got.String())
		})
	}
}

func TestEmail_Equal(t *testing.T) {
	t.Parallel()
	a, err := valueobjects.NewEmail("JOAO@EXAMPLE.COM")
	require.NoError(t, err)
	b, err := valueobjects.NewEmail("joao@example.com")
	require.NoError(t, err)
	c, err := valueobjects.NewEmail("maria@example.com")
	require.NoError(t, err)

	assert.True(t, a.Equal(b))
	assert.False(t, a.Equal(c))
}

func TestEmail_Masked(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"joao@example.com", "j***@example.com"},
		{"a@b.com", "a***@b.com"},
		{"UPPER@DOMAIN.ORG", "u***@domain.org"},
		{"z@x.io", "z***@x.io"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			e, err := valueobjects.NewEmail(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, e.Masked())
		})
	}
}

func TestEmail_MaskedZeroValue(t *testing.T) {
	t.Parallel()
	var e valueobjects.Email
	masked := e.Masked()
	assert.Equal(t, "***", masked)
}
