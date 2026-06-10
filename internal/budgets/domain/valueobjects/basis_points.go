package valueobjects

import (
	"errors"
	"fmt"
)

var ErrBasisPointsOutOfRange = errors.New("budgets: basis points fora do intervalo 0..10000")

type BasisPoints struct {
	value int
}

func NewBasisPoints(v int) (BasisPoints, error) {
	if v < 0 || v > 10000 {
		return BasisPoints{}, fmt.Errorf("budgets: %d: %w", v, ErrBasisPointsOutOfRange)
	}
	return BasisPoints{value: v}, nil
}

func (b BasisPoints) Int() int {
	return b.value
}

func (b BasisPoints) Add(other BasisPoints) (BasisPoints, error) {
	return NewBasisPoints(b.value + other.value)
}
