package input

import (
	"errors"

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
	var errs []error
	if !i.Kind.IsValid() {
		errs = append(errs, ErrInvalidKind)
	}
	if _, err := valueobjects.NewSearchQuery(i.Query); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (i *SearchDictionaryInput) NormalizedQuery() string {
	return valueobjects.NormalizeSearchQuery(i.Query)
}

func (i *SearchDictionaryInput) SearchQuery() (valueobjects.SearchQuery, error) {
	return valueobjects.NewSearchQuery(i.Query)
}
