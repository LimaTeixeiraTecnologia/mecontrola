package valueobjects

import (
	"errors"
	"fmt"
)

var ErrInvalidConfidence = errors.New("categories: invalid confidence")

type Confidence uint8

const (
	ConfidenceHigh Confidence = iota + 1
	ConfidenceMedium
	ConfidenceLow
)

func ParseConfidence(s string) (Confidence, error) {
	switch s {
	case "high":
		return ConfidenceHigh, nil
	case "medium":
		return ConfidenceMedium, nil
	case "low":
		return ConfidenceLow, nil
	default:
		return 0, fmt.Errorf("categories: %q: %w", s, ErrInvalidConfidence)
	}
}

func (c Confidence) String() string {
	switch c {
	case ConfidenceHigh:
		return "high"
	case ConfidenceMedium:
		return "medium"
	case ConfidenceLow:
		return "low"
	default:
		return ""
	}
}

func (c Confidence) IsValid() bool {
	switch c {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		return true
	default:
		return false
	}
}
