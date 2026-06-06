package valueobjects

import (
	"errors"
	"strings"
)

var ErrFunnelTokenEmpty = errors.New("billing: funnel token is empty")

type FunnelToken struct {
	value string
}

func NewFunnelToken(raw string) (FunnelToken, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return FunnelToken{}, ErrFunnelTokenEmpty
	}

	return FunnelToken{value: trimmed}, nil
}

func (t FunnelToken) String() string {
	return t.value
}
