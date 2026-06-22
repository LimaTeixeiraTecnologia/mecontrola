package input

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type ListDictionaryInput struct {
	CategoryID *string
	Kind       *valueobjects.Kind
	SignalType *valueobjects.SignalType
	Cursor     string
	PageSize   int
}

func (i *ListDictionaryInput) Validate() error {
	return nil
}
