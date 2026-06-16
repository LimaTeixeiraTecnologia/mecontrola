package valueobjects

import (
	"errors"
	"fmt"
)

var ErrCardDueDayOutOfRange = errors.New("onboarding: card due day out of range")

type CardDueDay struct {
	value int
}

func NewCardDueDay(day int) (CardDueDay, error) {
	if day < 1 || day > 31 {
		return CardDueDay{}, fmt.Errorf("onboarding: %d: %w (1..31)", day, ErrCardDueDayOutOfRange)
	}
	return CardDueDay{value: day}, nil
}

func (d CardDueDay) Value() int {
	return d.value
}
