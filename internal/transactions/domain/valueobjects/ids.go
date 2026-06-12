package valueobjects

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var ErrInvalidUserID = errors.New("transactions: user_id inválido")
var ErrInvalidCardID = errors.New("transactions: card_id inválido")
var ErrInvalidCategoryID = errors.New("transactions: category_id inválido")
var ErrInvalidSubcategoryID = errors.New("transactions: subcategory_id inválido")

type UserID struct{ value uuid.UUID }
type CardID struct{ value uuid.UUID }
type CategoryID struct{ value uuid.UUID }
type SubcategoryID struct{ value uuid.UUID }

func ParseUserID(s string) (UserID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return UserID{}, fmt.Errorf("transactions: %q: %w", s, ErrInvalidUserID)
	}
	return UserID{value: id}, nil
}

func UserIDFromUUID(id uuid.UUID) UserID {
	return UserID{value: id}
}

func (u UserID) UUID() uuid.UUID { return u.value }

func ParseCardID(s string) (CardID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return CardID{}, fmt.Errorf("transactions: %q: %w", s, ErrInvalidCardID)
	}
	return CardID{value: id}, nil
}

func CardIDFromUUID(id uuid.UUID) CardID {
	return CardID{value: id}
}

func (c CardID) UUID() uuid.UUID { return c.value }

func ParseCategoryID(s string) (CategoryID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return CategoryID{}, fmt.Errorf("transactions: %q: %w", s, ErrInvalidCategoryID)
	}
	return CategoryID{value: id}, nil
}

func CategoryIDFromUUID(id uuid.UUID) CategoryID {
	return CategoryID{value: id}
}

func (c CategoryID) UUID() uuid.UUID { return c.value }

func ParseSubcategoryID(s string) (SubcategoryID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return SubcategoryID{}, fmt.Errorf("transactions: %q: %w", s, ErrInvalidSubcategoryID)
	}
	return SubcategoryID{value: id}, nil
}

func SubcategoryIDFromUUID(id uuid.UUID) SubcategoryID {
	return SubcategoryID{value: id}
}

func (s SubcategoryID) UUID() uuid.UUID { return s.value }
