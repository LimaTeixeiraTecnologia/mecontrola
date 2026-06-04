package valueobjects_test

import (
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

// FuzzNewWhatsAppNumber garante que nenhum input arbitrário provoca panic.
func FuzzNewWhatsAppNumber(f *testing.F) {
	// corpus seed cobrindo edges
	seeds := []string{
		"",
		"   ",
		"11988887777",
		"+5511988887777",
		"5511988887777",
		"551188887777",
		"(11) 98888-7777",
		"11-98888-7777",
		"0000000000",
		"99999999999999",
		"abcdefghijk",
		"+1-800-555-0000",
		"441188887777",
		"あいうえお",
		"\x00\x01\x02",
		"11 9 8888-7777",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// nunca deve entrar em panic — erros são esperados para inputs inválidos
		_, _ = valueobjects.NewWhatsAppNumber(input)
	})
}
