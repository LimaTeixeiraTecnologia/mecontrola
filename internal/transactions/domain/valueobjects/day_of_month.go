package valueobjects

import (
	"errors"
	"fmt"
)

var ErrDayOfMonthOutOfRange = errors.New("transactions: day of month out of range")

type DayOfMonth struct {
	value int
	max   int
}

func NewDayOfMonth(day int) (DayOfMonth, error) {
	if day < 1 || day > 28 {
		return DayOfMonth{}, fmt.Errorf("transactions: %d: %w (1..28)", day, ErrDayOfMonthOutOfRange)
	}
	return DayOfMonth{value: day, max: 28}, nil
}

func NewDayOfMonthSnapshot(day int) (DayOfMonth, error) {
	if day < 1 || day > 31 {
		return DayOfMonth{}, fmt.Errorf("transactions: %d: %w (1..31)", day, ErrDayOfMonthOutOfRange)
	}
	return DayOfMonth{value: day, max: 31}, nil
}

func (d DayOfMonth) Value() int {
	return d.value
}
