package valueobjects

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type WhatsAppNumber struct{ e164 string }

var (
	ErrEmptyWhatsAppNumber   = errors.New("identity: whatsapp number vazio")
	ErrInvalidWhatsAppFormat = errors.New("identity: whatsapp number formato inválido (esperado BR E.164)")
	ErrUnsupportedCountry    = errors.New("identity: whatsapp number deve ser brasileiro (+55)")
)

var nonDigitPattern = regexp.MustCompile(`\D+`)

type whatsAppNormalizer struct{}

func (whatsAppNormalizer) KeepDigits(input string) string {
	return nonDigitPattern.ReplaceAllString(strings.TrimSpace(input), "")
}

// NormalizeBR aceita 10/11/12/13 dígitos após limpeza e devolve o conteúdo E.164 BR.
// Regras:
//   - 10 dígitos: DDD (2) + 8 → injeta 55 e o 9 nono dígito.
//   - 11 dígitos: DDD (2) + 9 + 8 → injeta 55.
//   - 12 dígitos começando 55: 55 + DDD + 8 → injeta o 9 nono dígito.
//   - 13 dígitos começando 55: já canônico.
func (whatsAppNormalizer) NormalizeBR(digits string) (string, error) {
	if digits == "" {
		return "", ErrEmptyWhatsAppNumber
	}
	switch len(digits) {
	case 10:
		return "55" + digits[:2] + "9" + digits[2:], nil
	case 11:
		return "55" + digits, nil
	case 12:
		if !strings.HasPrefix(digits, "55") {
			return "", ErrUnsupportedCountry
		}
		return digits[:4] + "9" + digits[4:], nil
	case 13:
		if !strings.HasPrefix(digits, "55") {
			return "", ErrUnsupportedCountry
		}
		return digits, nil
	default:
		return "", fmt.Errorf("%w: %d dígitos", ErrInvalidWhatsAppFormat, len(digits))
	}
}

func NewWhatsAppNumber(input string) (WhatsAppNumber, error) {
	normalizer := whatsAppNormalizer{}
	digits := normalizer.KeepDigits(input)
	normalized, err := normalizer.NormalizeBR(digits)
	if err != nil {
		return WhatsAppNumber{}, err
	}
	return WhatsAppNumber{e164: "+" + normalized}, nil
}

func (n WhatsAppNumber) String() string                   { return n.e164 }
func (n WhatsAppNumber) IsZero() bool                     { return n.e164 == "" }
func (n WhatsAppNumber) Equals(other WhatsAppNumber) bool { return n.e164 == other.e164 }
