package entities

import (
	billingdomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain"
)

type SubscriptionID struct{ value string }

func NewSubscriptionID(v string) (SubscriptionID, error) {
	if v == "" {
		return SubscriptionID{}, billingdomain.ErrInvalidSubscriptionID
	}
	return SubscriptionID{value: v}, nil
}

func (s SubscriptionID) String() string { return s.value }
func (s SubscriptionID) IsZero() bool   { return s.value == "" }
