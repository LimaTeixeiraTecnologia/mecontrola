package sanitize

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrEmpty = errors.New("agent.sanitize: text is empty after sanitization")

const DefaultMaxRunes = 2000

const (
	maskCard = "[REDACTED_CARD]"
	maskCPF  = "[REDACTED_CPF]"
)

const (
	cpfExpr  = `\b\d{3}\.\d{3}\.\d{3}-\d{2}\b`
	cardExpr = `\b\d(?:[ -]?\d){12,18}\b`
)

type Sanitizer struct {
	maxRunes int
	cpf      *regexp.Regexp
	card     *regexp.Regexp
}

func NewSanitizer(maxRunes int) (*Sanitizer, error) {
	if maxRunes <= 0 {
		maxRunes = DefaultMaxRunes
	}
	cpf, err := regexp.Compile(cpfExpr)
	if err != nil {
		return nil, fmt.Errorf("agent.sanitize: compile cpf: %w", err)
	}
	card, err := regexp.Compile(cardExpr)
	if err != nil {
		return nil, fmt.Errorf("agent.sanitize: compile card: %w", err)
	}
	return &Sanitizer{maxRunes: maxRunes, cpf: cpf, card: card}, nil
}

func (s *Sanitizer) Clean(raw string) (string, error) {
	normalized := s.normalizeControl(raw)
	if normalized == "" {
		return "", ErrEmpty
	}
	masked := s.cpf.ReplaceAllString(normalized, maskCPF)
	masked = s.card.ReplaceAllString(masked, maskCard)
	return s.capRunes(masked), nil
}

func (s *Sanitizer) normalizeControl(in string) string {
	mapped := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) || !utf8.ValidRune(r) {
			return -1
		}
		return r
	}, in)
	return strings.Join(strings.Fields(mapped), " ")
}

func (s *Sanitizer) capRunes(in string) string {
	if utf8.RuneCountInString(in) <= s.maxRunes {
		return in
	}
	runes := []rune(in)
	return strings.TrimSpace(string(runes[:s.maxRunes]))
}
