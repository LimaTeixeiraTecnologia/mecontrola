package valueobjects

import (
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

const maxCardLimitCents int64 = 100_000_000

type CardLimit struct {
	cents int64
}

func NewCardLimit(cents int64) (CardLimit, error) {
	if cents < 0 {
		return CardLimit{}, domain.ErrCardLimitNegative
	}
	if cents > maxCardLimitCents {
		return CardLimit{}, domain.ErrCardLimitTooLarge
	}
	return CardLimit{cents: cents}, nil
}

func (c CardLimit) Cents() int64 {
	return c.cents
}

func (c CardLimit) IsZero() bool {
	return c.cents == 0
}
