package valueobjects

import (
	"errors"
	"fmt"
)

var ErrInvalidMatchQuality = errors.New("categories: invalid match quality")

type MatchQuality uint8

const (
	MatchQualityExact MatchQuality = iota + 1
	MatchQualityToken
	MatchQualityFuzzy
)

func ParseMatchQuality(s string) (MatchQuality, error) {
	switch s {
	case "exact":
		return MatchQualityExact, nil
	case "token":
		return MatchQualityToken, nil
	case "fuzzy":
		return MatchQualityFuzzy, nil
	default:
		return 0, fmt.Errorf("categories: %q: %w", s, ErrInvalidMatchQuality)
	}
}

func (q MatchQuality) String() string {
	switch q {
	case MatchQualityExact:
		return "exact"
	case MatchQualityToken:
		return "token"
	case MatchQualityFuzzy:
		return "fuzzy"
	default:
		return ""
	}
}

func (q MatchQuality) IsValid() bool {
	switch q {
	case MatchQualityExact, MatchQualityToken, MatchQualityFuzzy:
		return true
	default:
		return false
	}
}

func (q MatchQuality) Weight() float64 {
	switch q {
	case MatchQualityExact:
		return 1.0
	case MatchQualityToken:
		return 0.8
	case MatchQualityFuzzy:
		return 0.5
	default:
		return 0.0
	}
}
