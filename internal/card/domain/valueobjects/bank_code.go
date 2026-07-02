package valueobjects

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

type BankCode struct{ display string }

func NewBankCode(raw string) (BankCode, error) {
	display := strings.TrimSpace(raw)
	if display == "" || normalizeBank(display) == "" {
		return BankCode{}, domain.ErrInvalidBank
	}
	return BankCode{display: display}, nil
}

func (b BankCode) String() string    { return b.display }
func (b BankCode) LookupKey() string { return normalizeBank(b.display) }

func normalizeBank(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, strings.ToLower(strings.TrimSpace(s)))
	parts := strings.Fields(result)
	return strings.Join(parts, "-")
}
