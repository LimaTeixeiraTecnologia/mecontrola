package valueobjects

import (
	"unicode/utf8"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

type CardName struct {
	value string
}

func NewCardName(v string) (CardName, error) {
	n := utf8.RuneCountInString(v)
	if n < 1 || n > 64 {
		return CardName{}, domain.ErrInvalidCardName
	}
	return CardName{value: v}, nil
}

func (c CardName) String() string {
	return c.value
}
