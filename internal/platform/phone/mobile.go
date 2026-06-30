package phone

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrMobileEmpty = errors.New("phone: mobile is empty")

var ErrMobileInvalid = errors.New("phone: mobile invalid for BR E.164")

var brCellPattern = regexp.MustCompile(`^\+55\d{2}9\d{8}$`)

type Mobile struct {
	e164 string
}

func NewMobileBR(raw string) (Mobile, error) {
	e164, err := NormalizeBR(raw)
	if err != nil {
		return Mobile{}, err
	}
	return Mobile{e164: e164}, nil
}

func (m Mobile) String() string { return m.e164 }

func NormalizeBR(raw string) (string, error) {
	s := normalizeRaw(raw)
	if s == "" {
		return "", ErrMobileEmpty
	}
	if !brCellPattern.MatchString(s) {
		return "", fmt.Errorf("phone: %q: %w", raw, ErrMobileInvalid)
	}
	return s, nil
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
