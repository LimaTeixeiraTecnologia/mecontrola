package valueobjects

import (
	"errors"
	"strings"
)

const minSearchQueryLen = 2

var ErrSearchQueryTooShort = errors.New("transactions: search query must have at least 2 characters")

type SearchQuery struct {
	value string
}

func NewSearchQuery(s string) (SearchQuery, error) {
	trimmed := strings.TrimSpace(s)
	if len([]rune(trimmed)) < minSearchQueryLen {
		return SearchQuery{}, ErrSearchQueryTooShort
	}
	return SearchQuery{value: trimmed}, nil
}

func (q SearchQuery) String() string {
	return q.value
}
