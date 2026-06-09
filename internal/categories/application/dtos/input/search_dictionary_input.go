package input

import (
	"strings"
	"unicode"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

var (
	ErrInvalidKind  = valueobjects.ErrInvalidKind
	ErrInvalidQuery = valueobjects.ErrInvalidQuery
)

type SearchDictionaryInput struct {
	Query string
	Kind  valueobjects.Kind
}

func (i *SearchDictionaryInput) Validate() error {
	if !i.Kind.IsValid() {
		return ErrInvalidKind
	}

	normalized := i.normalizeQuery(i.Query)
	if len(normalized) < 3 {
		return ErrInvalidQuery
	}

	return nil
}

func (i *SearchDictionaryInput) NormalizedQuery() string {
	return i.normalizeQuery(i.Query)
}

func (i *SearchDictionaryInput) normalizeQuery(q string) string {
	trimmed := strings.TrimSpace(q)
	var result strings.Builder
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			result.WriteRune(r)
		}
	}
	return strings.ToLower(result.String())
}
