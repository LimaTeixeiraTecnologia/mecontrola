package pii_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/pii"
)

func TestMaskDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "vazio",
			input: "",
			want:  "",
		},
		{
			name:  "1 rune ASCII",
			input: "A",
			want:  "*",
		},
		{
			name:  "1 rune multibyte (ç)",
			input: "ç",
			want:  "*",
		},
		{
			name:  "1 rune multibyte (é)",
			input: "é",
			want:  "*",
		},
		{
			name:  "2 runes ASCII",
			input: "Jo",
			want:  "J****",
		},
		{
			name:  "nome completo ASCII",
			input: "João Silva",
			want:  "J****",
		},
		{
			name:  "nome com acento inicial (Ângela)",
			input: "Ângela",
			want:  "Â****",
		},
		{
			name:  "nome apenas acentos multibyte",
			input: "érica",
			want:  "é****",
		},
		{
			name:  "byte inválido UTF-8",
			input: "\xff",
			want:  "****",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := pii.MaskDisplayName(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
