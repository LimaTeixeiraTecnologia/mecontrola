package valueobjects

import (
	"unicode/utf8"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

type Nickname struct {
	value string
}

func NewNickname(v string) (Nickname, error) {
	n := utf8.RuneCountInString(v)
	if n < 1 || n > 32 {
		return Nickname{}, domain.ErrInvalidNickname
	}
	return Nickname{value: v}, nil
}

func (n Nickname) String() string {
	return n.value
}
