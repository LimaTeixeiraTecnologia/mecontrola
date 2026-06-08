package valueobjects

import "errors"

var ErrTokenStatusInvalid = errors.New("onboarding: token status invalid")

type TokenStatus uint8

const (
	TokenStatusPending TokenStatus = iota + 1
	TokenStatusPaid
	TokenStatusConsumed
	TokenStatusExpired
)

func (s TokenStatus) String() string {
	switch s {
	case TokenStatusPending:
		return "PENDING"
	case TokenStatusPaid:
		return "PAID"
	case TokenStatusConsumed:
		return "CONSUMED"
	case TokenStatusExpired:
		return "EXPIRED"
	default:
		return "UNKNOWN"
	}
}

func ParseTokenStatus(raw string) (TokenStatus, error) {
	switch raw {
	case "PENDING":
		return TokenStatusPending, nil
	case "PAID":
		return TokenStatusPaid, nil
	case "CONSUMED":
		return TokenStatusConsumed, nil
	case "EXPIRED":
		return TokenStatusExpired, nil
	default:
		return 0, ErrTokenStatusInvalid
	}
}
