package input

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

var (
	ErrRootCategoryIDRequired = errors.New("root_category_id: obrigatório")
	ErrSubcategoryIDRequired  = errors.New("subcategory_id: obrigatório")
	ErrVersionRequired        = errors.New("expected_version: deve ser maior que zero")
)

type ResolveCategoryForWriteInput struct {
	RootCategoryID  uuid.UUID
	SubcategoryID   uuid.UUID
	Kind            valueobjects.Kind
	ExpectedVersion int64
}

func (i *ResolveCategoryForWriteInput) Validate() error {
	var errs []error
	if i.RootCategoryID == uuid.Nil {
		errs = append(errs, ErrRootCategoryIDRequired)
	}
	if i.SubcategoryID == uuid.Nil {
		errs = append(errs, ErrSubcategoryIDRequired)
	}
	if !i.Kind.IsValid() {
		errs = append(errs, ErrInvalidKind)
	}
	if i.ExpectedVersion <= 0 {
		errs = append(errs, ErrVersionRequired)
	}
	return errors.Join(errs...)
}
