package usecases

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

func ParseKind(s string) (valueobjects.Kind, error) {
	return valueobjects.ParseKind(s)
}
