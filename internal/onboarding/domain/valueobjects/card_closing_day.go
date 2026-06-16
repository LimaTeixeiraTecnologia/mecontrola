package valueobjects

import (
	"errors"
	"fmt"
)

var ErrCardClosingDayOutOfRange = errors.New("onboarding: card closing day out of range")

type CardClosingDay struct {
	value int
}

func NewCardClosingDay(day int) (CardClosingDay, error) {
	if day < 1 || day > 31 {
		return CardClosingDay{}, fmt.Errorf("onboarding: %d: %w (1..31)", day, ErrCardClosingDayOutOfRange)
	}
	return CardClosingDay{value: day}, nil
}

func (d CardClosingDay) Value() int {
	return d.value
}
