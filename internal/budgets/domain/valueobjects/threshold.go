package valueobjects

import (
	"errors"
	"fmt"
)

var ErrThresholdUnknown = errors.New("budgets: threshold desconhecido")

type Threshold uint8

const (
	Threshold80 Threshold = iota + 1
	Threshold100
)

func ParseThreshold(v int) (Threshold, error) {
	switch v {
	case 80:
		return Threshold80, nil
	case 100:
		return Threshold100, nil
	default:
		return 0, fmt.Errorf("budgets: %d: %w", v, ErrThresholdUnknown)
	}
}

func (t Threshold) Int() int {
	switch t {
	case Threshold80:
		return 80
	case Threshold100:
		return 100
	default:
		return 0
	}
}

func (t Threshold) String() string {
	switch t {
	case Threshold80:
		return "t80"
	case Threshold100:
		return "t100"
	default:
		return ""
	}
}
