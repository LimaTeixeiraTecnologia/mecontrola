package valueobjects

type MoneyBRL struct{ cents int64 }

func NewMoneyBRL(cents int64) (MoneyBRL, error) {
	if cents < 0 {
		return MoneyBRL{}, ErrNegativeAmount
	}
	return MoneyBRL{cents: cents}, nil
}

func (m MoneyBRL) Cents() int64 { return m.cents }
func (m MoneyBRL) IsZero() bool { return m.cents == 0 }
