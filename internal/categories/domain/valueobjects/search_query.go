package valueobjects

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	minSearchQueryLen = 3
	minTokenLen       = 2
)

var searchStopwords = map[string]struct{}{
	"de": {}, "da": {}, "do": {}, "das": {}, "dos": {},
	"no": {}, "na": {}, "nos": {}, "nas": {}, "em": {},
	"com": {}, "por": {}, "para": {}, "pra": {}, "pro": {},
	"um": {}, "uma": {}, "uns": {}, "umas": {}, "ao": {}, "aos": {},
	"e": {}, "ou": {}, "the": {},
	"hoje": {}, "ontem": {}, "amanha": {},
	"paguei": {}, "pagar": {}, "comprei": {}, "comprar": {},
	"gastei": {}, "gastar": {}, "recebi": {}, "receber": {},
	"meu": {}, "minha": {}, "meus": {}, "minhas": {},
}

type SearchQuery struct {
	raw        string
	normalized string
	tokens     []string
}

func NewSearchQuery(raw string) (SearchQuery, error) {
	normalized := normalizeSearchQuery(raw)
	if len(normalized) < minSearchQueryLen {
		return SearchQuery{}, ErrInvalidQuery
	}
	return SearchQuery{raw: raw, normalized: normalized, tokens: tokenizeSearchQuery(raw)}, nil
}

func NormalizeSearchQuery(raw string) string {
	return normalizeSearchQuery(raw)
}

func (q SearchQuery) Raw() string {
	return q.raw
}

func (q SearchQuery) Trimmed() string {
	return strings.TrimSpace(q.raw)
}

func (q SearchQuery) Normalized() string {
	return q.normalized
}

func (q SearchQuery) Tokens() []string {
	return q.tokens
}

func normalizeSearchQuery(raw string) string {
	trimmed := strings.TrimSpace(raw)
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

func tokenizeSearchQuery(raw string) []string {
	lowered := strings.ToLower(strings.TrimSpace(raw))
	fields := strings.FieldsFunc(lowered, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	tokens := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		token := unaccent(field)
		if len([]rune(token)) < minTokenLen {
			continue
		}
		if _, stop := searchStopwords[token]; stop {
			continue
		}
		if _, dup := seen[token]; dup {
			continue
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}
	return tokens
}

func unaccent(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return norm.NFC.String(b.String())
}
