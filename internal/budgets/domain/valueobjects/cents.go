package valueobjects

import (
	"errors"
	"fmt"
)

var ErrCentsNonPositive = errors.New("budgets: valor em centavos deve ser maior que zero")

type Cents struct {
	value int64
}

func NewCents(v int64) (Cents, error) {
	if v <= 0 {
		return Cents{}, fmt.Errorf("budgets: %d: %w", v, ErrCentsNonPositive)
	}
	return Cents{value: v}, nil
}

func CentsFromInt64(v int64) Cents {
	return Cents{value: v}
}

func (c Cents) Int64() int64 {
	return c.value
}

func (c Cents) IsPositive() bool {
	return c.value > 0
}

func (c Cents) Add(other Cents) Cents {
	return Cents{value: c.value + other.value}
}
