package valueobjects

import (
	"errors"
	"fmt"
)

const maxDescriptionLen = 500

var ErrDescriptionEmpty = errors.New("transactions: description must not be empty")
var ErrDescriptionTooLong = errors.New("transactions: description too long")

type Description struct {
	value string
}

func NewDescription(s string) (Description, error) {
	if s == "" {
		return Description{}, ErrDescriptionEmpty
	}
	if len(s) > maxDescriptionLen {
		return Description{}, fmt.Errorf("transactions: length %d exceeds %d: %w", len(s), maxDescriptionLen, ErrDescriptionTooLong)
	}
	return Description{value: s}, nil
}

func (d Description) String() string {
	return d.value
}
