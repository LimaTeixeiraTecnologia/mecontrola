package valueobjects

import (
	"errors"
	"fmt"
)

var ErrMoneyMustBePositive = errors.New("transactions: money must be greater than zero")

type Money struct {
	cents int64
}

func NewMoney(cents int64) (Money, error) {
	if cents <= 0 {
		return Money{}, fmt.Errorf("transactions: %d: %w", cents, ErrMoneyMustBePositive)
	}
	return Money{cents: cents}, nil
}

func (m Money) Cents() int64 {
	return m.cents
}

func (m Money) Equal(other Money) bool {
	return m.cents == other.cents
}
