package interfaces

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type DictionaryQuery struct {
	CategoryID *string
	Kind       *valueobjects.Kind
	SignalType *valueobjects.SignalType
	Cursor     string
	PageSize   int
}

type DictionarySearchQuery struct {
	Kind              valueobjects.Kind
	Term              string
	Limit             int
	IncludeDeprecated bool
}

type DictionaryRepository interface {
	List(ctx context.Context, q DictionaryQuery) ([]entities.DictionaryEntry, string, error)
	Search(ctx context.Context, q DictionarySearchQuery) ([]entities.DictionaryEntry, error)
}
