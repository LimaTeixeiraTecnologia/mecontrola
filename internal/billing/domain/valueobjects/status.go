package valueobjects

import (
	"errors"
	"fmt"
)

var ErrUnknownStatus = errors.New("billing: unknown subscription status")

type Status uint8

const (
	StatusTrialing Status = iota + 1
	StatusActive
	StatusPastDue
	StatusCanceledPending
	StatusExpired
	StatusRefunded
)

func (s Status) IsActiveForBilling() bool {
	switch s {
	case StatusActive, StatusPastDue, StatusCanceledPending:
		return true
	default:
		return false
	}
}

func (s Status) IsTerminal() bool {
	switch s {
	case StatusExpired, StatusRefunded:
		return true
	default:
		return false
	}
}

func ParseStatus(s string) (Status, error) {
	switch s {
	case "TRIALING":
		return StatusTrialing, nil
	case "ACTIVE":
		return StatusActive, nil
	case "PAST_DUE":
		return StatusPastDue, nil
	case "CANCELED_PENDING":
		return StatusCanceledPending, nil
	case "EXPIRED":
		return StatusExpired, nil
	case "REFUNDED":
		return StatusRefunded, nil
	default:
		return 0, fmt.Errorf("billing: %q: %w", s, ErrUnknownStatus)
	}
}

func (s Status) String() string {
	switch s {
	case StatusTrialing:
		return "TRIALING"
	case StatusActive:
		return "ACTIVE"
	case StatusPastDue:
		return "PAST_DUE"
	case StatusCanceledPending:
		return "CANCELED_PENDING"
	case StatusExpired:
		return "EXPIRED"
	case StatusRefunded:
		return "REFUNDED"
	default:
		return ""
	}
}
