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

type statusInfo struct {
	wire             string
	activeForBilling bool
	terminal         bool
}

var statusTable = map[Status]statusInfo{
	StatusTrialing:        {wire: "TRIALING"},
	StatusActive:          {wire: "ACTIVE", activeForBilling: true},
	StatusPastDue:         {wire: "PAST_DUE", activeForBilling: true},
	StatusCanceledPending: {wire: "CANCELED_PENDING", activeForBilling: true},
	StatusExpired:         {wire: "EXPIRED", terminal: true},
	StatusRefunded:        {wire: "REFUNDED", terminal: true},
}

var statusByWire = func() map[string]Status {
	m := make(map[string]Status, len(statusTable))
	for s, info := range statusTable {
		m[info.wire] = s
	}
	return m
}()

func (s Status) String() string           { return statusTable[s].wire }
func (s Status) IsActiveForBilling() bool { return statusTable[s].activeForBilling }
func (s Status) IsTerminal() bool         { return statusTable[s].terminal }

func ParseStatus(s string) (Status, error) {
	if st, ok := statusByWire[s]; ok {
		return st, nil
	}
	return 0, fmt.Errorf("billing: %q: %w", s, ErrUnknownStatus)
}
