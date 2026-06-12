package valueobjects

import (
	"strings"
	"unicode"
)

const minSearchQueryLen = 3

type SearchQuery struct {
	raw        string
	normalized string
}

func NewSearchQuery(raw string) (SearchQuery, error) {
	normalized := normalizeSearchQuery(raw)
	if len(normalized) < minSearchQueryLen {
		return SearchQuery{}, ErrInvalidQuery
	}
	return SearchQuery{raw: raw, normalized: normalized}, nil
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
