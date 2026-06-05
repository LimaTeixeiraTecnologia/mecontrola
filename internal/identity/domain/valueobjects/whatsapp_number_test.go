package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func TestNewWhatsAppNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantE164    string
		wantErr     error
		wantErrWrap bool
	}{
		{
			name:     "e164 canônico",
			input:    "+5511988887777",
			wantE164: "+5511988887777",
		},
		{
			name:     "sem +55",
			input:    "11988887777",
			wantE164: "+5511988887777",
		},
		{
			name:     "com 55 sem +",
			input:    "5511988887777",
			wantE164: "+5511988887777",
		},
		{
			name:     "formato (11) 98888-7777",
			input:    "(11) 98888-7777",
			wantE164: "+5511988887777",
		},
		{
			name:     "formato 11 98888-7777",
			input:    "11 98888-7777",
			wantE164: "+5511988887777",
		},
		{
			name:     "ddd 21",
			input:    "+5521987654321",
			wantE164: "+5521987654321",
		},
		{
			name:    "vazio",
			input:   "",
			wantErr: valueobjects.ErrWhatsAppNumberEmpty,
		},
		{
			name:    "apenas espaços",
			input:   "   ",
			wantErr: valueobjects.ErrWhatsAppNumberEmpty,
		},
		{
			name:        "fixo sem 9",
			input:       "+551133334444",
			wantErrWrap: true,
			wantErr:     valueobjects.ErrWhatsAppNumberInvalid,
		},
		{
			name:        "celular com 7 dígitos finais",
			input:       "+55119888777",
			wantErrWrap: true,
			wantErr:     valueobjects.ErrWhatsAppNumberInvalid,
		},
		{
			name:        "celular com 9 dígitos finais (11 no total após DDD)",
			input:       "+551198888777777",
			wantErrWrap: true,
			wantErr:     valueobjects.ErrWhatsAppNumberInvalid,
		},
		{
			name:        "país diferente",
			input:       "+5511988887777extra",
			wantErrWrap: true,
			wantErr:     valueobjects.ErrWhatsAppNumberInvalid,
		},
		{
			name:        "DDD com 3 dígitos",
			input:       "+55119988887777",
			wantErrWrap: true,
			wantErr:     valueobjects.ErrWhatsAppNumberInvalid,
		},
		{
			name:        "unicode no número",
			input:       "+55119888é7777",
			wantErrWrap: true,
			wantErr:     valueobjects.ErrWhatsAppNumberInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := valueobjects.NewWhatsAppNumber(tc.input)

			if tc.wantErr != nil {
				require.Error(t, err)
				if tc.wantErrWrap {
					assert.True(t, errors.Is(err, tc.wantErr), "erro deve envolver %v, got %v", tc.wantErr, err)
				} else {
					assert.ErrorIs(t, err, tc.wantErr)
				}
				assert.Equal(t, valueobjects.WhatsAppNumber{}, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantE164, got.String())
		})
	}
}

func TestWhatsAppNumber_Equal(t *testing.T) {
	t.Parallel()
	a, err := valueobjects.NewWhatsAppNumber("+5511988887777")
	require.NoError(t, err)
	b, err := valueobjects.NewWhatsAppNumber("(11) 98888-7777")
	require.NoError(t, err)
	c, err := valueobjects.NewWhatsAppNumber("+5521987654321")
	require.NoError(t, err)

	assert.True(t, a.Equal(b))
	assert.False(t, a.Equal(c))
}

func TestWhatsAppNumber_Masked(t *testing.T) {
	t.Parallel()
	n, err := valueobjects.NewWhatsAppNumber("+5511988887777")
	require.NoError(t, err)
	masked := n.Masked()
	assert.Equal(t, "+55 11 9****-7777", masked)
}

func TestWhatsAppNumber_MaskedZeroValue(t *testing.T) {
	t.Parallel()
	var n valueobjects.WhatsAppNumber
	masked := n.Masked()
	assert.Equal(t, "****", masked)
}
