package tools

import (
	"errors"
	"strings"
)

const maxEntryAmountCents int64 = 1_000_000_000

var (
	errAmountNonPositive  = errors.New("amount_non_positive")
	errAmountAboveCeiling = errors.New("amount_above_ceiling")
	errDescriptionEmpty   = errors.New("description_empty")
)

func validateEntryAmount(cents int64) error {
	if cents <= 0 {
		return errAmountNonPositive
	}
	if cents > maxEntryAmountCents {
		return errAmountAboveCeiling
	}
	return nil
}

func validateEntryDescription(description string) error {
	if strings.TrimSpace(description) == "" {
		return errDescriptionEmpty
	}
	return nil
}
