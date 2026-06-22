package valueobjects

import "errors"

var ErrConfidenceOutOfRange = errors.New("agent.intent: confidence must be between 0 and 1")

type Confidence struct {
	value float64
}

func NewConfidence(raw float64) (Confidence, error) {
	if raw < 0 || raw > 1 {
		return Confidence{}, ErrConfidenceOutOfRange
	}
	return Confidence{value: raw}, nil
}

func (c Confidence) Value() float64 { return c.value }

func (c Confidence) Below(threshold Confidence) bool { return c.value < threshold.value }
