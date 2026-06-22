package input

import (
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type ListCategoriesInput struct {
	Kind              *valueobjects.Kind
	ParentID          *uuid.UUID
	IncludeDeprecated bool
}

func (i *ListCategoriesInput) Validate() error {
	return nil
}
