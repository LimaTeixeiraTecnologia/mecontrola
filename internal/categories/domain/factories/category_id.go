package factories

import (
	"github.com/google/uuid"
)

var categoryNamespace = uuid.NewSHA1(uuid.Nil, []byte("mecontrola.io/categories"))

func NewCategoryID(kind, slug string) uuid.UUID {
	return uuid.NewSHA1(categoryNamespace, []byte(kind+":"+slug))
}
