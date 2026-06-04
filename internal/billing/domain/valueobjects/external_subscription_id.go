package valueobjects

import "strings"

type ExternalSubscriptionID struct{ value string }

func NewExternalSubscriptionID(v string) (ExternalSubscriptionID, error) {
	if strings.TrimSpace(v) == "" {
		return ExternalSubscriptionID{}, ErrEmptyExternalSubscriptionID
	}
	return ExternalSubscriptionID{value: v}, nil
}

func (e ExternalSubscriptionID) String() string { return e.value }
func (e ExternalSubscriptionID) IsZero() bool   { return e.value == "" }
