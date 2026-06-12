package input

import (
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
	if _, err := valueobjects.NewSearchQuery(i.Query); err != nil {
		return err
	}
	return nil
}

func (i *SearchDictionaryInput) NormalizedQuery() string {
	return valueobjects.NormalizeSearchQuery(i.Query)
}

func (i *SearchDictionaryInput) SearchQuery() (valueobjects.SearchQuery, error) {
	return valueobjects.NewSearchQuery(i.Query)
}
