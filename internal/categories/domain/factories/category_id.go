package factories

import (
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

var categoryNamespace = uuid.NewSHA1(uuid.Nil, []byte("mecontrola.io/categories"))

func NewCategoryID(kind string, slug valueobjects.Slug) uuid.UUID {
	return uuid.NewSHA1(categoryNamespace, []byte(kind+":"+slug.String()))
}
