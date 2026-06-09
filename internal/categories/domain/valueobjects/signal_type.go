package valueobjects

import (
	"errors"
	"fmt"
)

var ErrInvalidSignalType = errors.New("categories: invalid signal type")

type SignalType uint8

const (
	SignalTypeCanonicalName SignalType = iota + 1
	SignalTypeAlias
	SignalTypePhrase
	SignalTypeMerchant
	SignalTypeSegment
)

func ParseSignalType(s string) (SignalType, error) {
	switch s {
	case "canonical_name":
		return SignalTypeCanonicalName, nil
	case "alias":
		return SignalTypeAlias, nil
	case "phrase":
		return SignalTypePhrase, nil
	case "merchant":
		return SignalTypeMerchant, nil
	case "segment":
		return SignalTypeSegment, nil
	default:
		return 0, fmt.Errorf("categories: %q: %w", s, ErrInvalidSignalType)
	}
}

func (s SignalType) String() string {
	switch s {
	case SignalTypeCanonicalName:
		return "canonical_name"
	case SignalTypeAlias:
		return "alias"
	case SignalTypePhrase:
		return "phrase"
	case SignalTypeMerchant:
		return "merchant"
	case SignalTypeSegment:
		return "segment"
	default:
		return ""
	}
}

func (s SignalType) Precedence() int {
	switch s {
	case SignalTypeCanonicalName:
		return 5
	case SignalTypeAlias:
		return 4
	case SignalTypePhrase:
		return 3
	case SignalTypeMerchant:
		return 2
	case SignalTypeSegment:
		return 1
	default:
		return 0
	}
}

func (s SignalType) IsValid() bool {
	switch s {
	case SignalTypeCanonicalName, SignalTypeAlias, SignalTypePhrase, SignalTypeMerchant, SignalTypeSegment:
		return true
	default:
		return false
	}
}
