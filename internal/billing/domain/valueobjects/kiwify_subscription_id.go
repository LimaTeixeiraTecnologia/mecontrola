package valueobjects

import (
	"errors"
	"strings"
)

var ErrKiwifySubscriptionIDEmpty = errors.New("billing: kiwify subscription id is empty")

type KiwifySubscriptionID struct {
	value string
}

func NewKiwifySubscriptionID(raw string) (KiwifySubscriptionID, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return KiwifySubscriptionID{}, ErrKiwifySubscriptionIDEmpty
	}

	return KiwifySubscriptionID{value: trimmed}, nil
}

func (id KiwifySubscriptionID) String() string {
	return id.value
}
