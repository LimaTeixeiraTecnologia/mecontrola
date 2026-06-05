package valueobjects

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrWhatsAppNumberEmpty = errors.New("identity: whatsapp number is empty")

var ErrWhatsAppNumberInvalid = errors.New("identity: whatsapp number invalid for BR E.164")

var brCellPattern = regexp.MustCompile(`^\+55\d{2}9\d{8}$`)

type WhatsAppNumber struct {
	e164 string
}

func NewWhatsAppNumber(raw string) (WhatsAppNumber, error) {
	cleaned := normalizeRaw(raw)
	if cleaned == "" {
		return WhatsAppNumber{}, ErrWhatsAppNumberEmpty
	}
	if !brCellPattern.MatchString(cleaned) {
		return WhatsAppNumber{}, fmt.Errorf("identity: %q: %w", raw, ErrWhatsAppNumberInvalid)
	}
	return WhatsAppNumber{e164: cleaned}, nil
}

func (w WhatsAppNumber) String() string { return w.e164 }

func (w WhatsAppNumber) Equal(o WhatsAppNumber) bool { return w.e164 == o.e164 }

func (w WhatsAppNumber) Masked() string {
	if len(w.e164) < 14 {
		return "****"
	}
	country := w.e164[0:3]
	ddd := w.e164[3:5]
	nineDigit := w.e164[5:6]
	last4 := w.e164[10:14]
	return country + " " + ddd + " " + nineDigit + "****-" + last4
}

func normalizeRaw(raw string) string {
	if raw == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "", "(", "", ")", "", "-", "")
	s := replacer.Replace(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(s, "+55"):
	case strings.HasPrefix(s, "55") && len(s) >= 12:
		s = "+" + s
	default:
		s = "+55" + s
	}
	return s
}
