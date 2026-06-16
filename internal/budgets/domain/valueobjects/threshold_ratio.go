package valueobjects

import (
	"errors"
	"fmt"
)

var ErrThresholdRatioOutOfRange = errors.New("budgets: threshold ratio fora do intervalo (0, 1]")

type ThresholdRatio struct {
	value float64
}

func NewThresholdRatio(v float64) (ThresholdRatio, error) {
	if v <= 0 || v > 1 {
		return ThresholdRatio{}, fmt.Errorf("budgets: ratio=%v: %w", v, ErrThresholdRatioOutOfRange)
	}
	return ThresholdRatio{value: v}, nil
}

func MustThresholdRatio(v float64) ThresholdRatio {
	r, err := NewThresholdRatio(v)
	if err != nil {
		return ThresholdRatio{value: 0}
	}
	return r
}

func (r ThresholdRatio) Float64() float64 {
	return r.value
}

func (r ThresholdRatio) IsZero() bool {
	return r.value == 0
}
