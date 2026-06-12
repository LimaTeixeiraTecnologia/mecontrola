package valueobjects

import (
	"errors"
	"fmt"
)

var ErrDirectionUnknown = errors.New("transactions: direction desconhecido (income ou outcome)")

type Direction uint8

const (
	DirectionIncome Direction = iota + 1
	DirectionOutcome
)

func ParseDirection(s string) (Direction, error) {
	switch s {
	case "income":
		return DirectionIncome, nil
	case "outcome":
		return DirectionOutcome, nil
	default:
		return 0, fmt.Errorf("transactions: %q: %w", s, ErrDirectionUnknown)
	}
}

func DirectionFromInt(v int) (Direction, error) {
	switch v {
	case 1:
		return DirectionIncome, nil
	case 2:
		return DirectionOutcome, nil
	default:
		return 0, fmt.Errorf("transactions: direction int %d: %w", v, ErrDirectionUnknown)
	}
}

func (d Direction) String() string {
	switch d {
	case DirectionIncome:
		return "income"
	case DirectionOutcome:
		return "outcome"
	default:
		return ""
	}
}
