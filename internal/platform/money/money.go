package money

import (
	"fmt"
	"strconv"
)

const basisPointsScale = 10000

type Money struct {
	cents int64
}

func FromCents(cents int64) Money {
	return Money{cents: cents}
}

func (m Money) Cents() int64 {
	return m.cents
}

func (m Money) Add(other Money) Money {
	return Money{cents: m.cents + other.cents}
}

func (m Money) Sub(other Money) Money {
	return Money{cents: m.cents - other.cents}
}

func (m Money) Abs() Money {
	if m.cents < 0 {
		return Money{cents: -m.cents}
	}
	return m
}

func (m Money) IsZero() bool {
	return m.cents == 0
}

func (m Money) ApplyBasisPoints(basisPoints int) Money {
	return Money{cents: RoundHalfEvenDiv(m.cents*int64(basisPoints), basisPointsScale)}
}

func (m Money) BasisPointsOf(total Money) int {
	if total.cents == 0 {
		return 0
	}
	return int(RoundHalfEvenDiv(m.cents*basisPointsScale, total.cents))
}

func (m Money) Amount() string {
	cents := m.cents
	if cents < 0 {
		cents = -cents
	}
	return fmt.Sprintf("%s,%02d", groupThousands(cents/100), cents%100)
}

func (m Money) BRL() string {
	return "R$ " + m.Amount()
}

func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	rem := len(s) % 3
	var out []byte
	if rem > 0 {
		out = append(out, s[:rem]...)
		out = append(out, '.')
	}
	for i := rem; i < len(s); i += 3 {
		out = append(out, s[i:i+3]...)
		if i+3 < len(s) {
			out = append(out, '.')
		}
	}
	return string(out)
}

func RoundHalfEvenDiv(numerator, denominator int64) int64 {
	if denominator == 0 {
		return 0
	}
	if denominator < 0 {
		numerator, denominator = -numerator, -denominator
	}
	negative := numerator < 0
	magnitude := numerator
	if negative {
		magnitude = -magnitude
	}
	quotient := magnitude / denominator
	remainder := magnitude % denominator
	twice := remainder * 2
	rounded := quotient
	if twice > denominator || (twice == denominator && quotient%2 == 1) {
		rounded = quotient + 1
	}
	if negative {
		return -rounded
	}
	return rounded
}
