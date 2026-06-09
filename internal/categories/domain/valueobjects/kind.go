package valueobjects

import (
	"errors"
	"fmt"
)

var ErrInvalidKind = errors.New("categories: invalid kind")

type Kind uint8

const (
	KindIncome Kind = iota + 1
	KindExpense
)

func ParseKind(s string) (Kind, error) {
	switch s {
	case "income":
		return KindIncome, nil
	case "expense":
		return KindExpense, nil
	default:
		return 0, fmt.Errorf("categories: %q: %w", s, ErrInvalidKind)
	}
}

func (k Kind) String() string {
	switch k {
	case KindIncome:
		return "income"
	case KindExpense:
		return "expense"
	default:
		return ""
	}
}

func (k Kind) IsValid() bool {
	switch k {
	case KindIncome, KindExpense:
		return true
	default:
		return false
	}
}
