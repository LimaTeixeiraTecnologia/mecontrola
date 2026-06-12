package valueobjects

import (
	"errors"
	"fmt"
)

var ErrFrequencyUnknown = errors.New("transactions: frequency desconhecida (monthly ou yearly)")

type Frequency uint8

const (
	FrequencyMonthly Frequency = iota + 1
	FrequencyYearly
)

func ParseFrequency(s string) (Frequency, error) {
	switch s {
	case "monthly":
		return FrequencyMonthly, nil
	case "yearly":
		return FrequencyYearly, nil
	default:
		return 0, fmt.Errorf("transactions: %q: %w", s, ErrFrequencyUnknown)
	}
}

func FrequencyFromInt(v int) (Frequency, error) {
	switch Frequency(v) {
	case FrequencyMonthly:
		return FrequencyMonthly, nil
	case FrequencyYearly:
		return FrequencyYearly, nil
	default:
		return 0, fmt.Errorf("transactions: %d: %w", v, ErrFrequencyUnknown)
	}
}

func (f Frequency) String() string {
	switch f {
	case FrequencyMonthly:
		return "monthly"
	case FrequencyYearly:
		return "yearly"
	default:
		return ""
	}
}
