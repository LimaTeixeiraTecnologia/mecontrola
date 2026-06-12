package valueobjects

import (
	"errors"
	"fmt"
)

const (
	minSlugLen = 2
	maxSlugLen = 64
)

var (
	ErrSlugEmpty        = errors.New("categories: slug must not be empty")
	ErrSlugTooShort     = errors.New("categories: slug too short")
	ErrSlugTooLong      = errors.New("categories: slug too long")
	ErrSlugInvalidChars = errors.New("categories: slug must be kebab-case (lowercase a-z, 0-9, hyphen)")
	ErrSlugEdgeHyphen   = errors.New("categories: slug must not start or end with hyphen")
	ErrSlugDoubleHyphen = errors.New("categories: slug must not contain consecutive hyphens")
)

type Slug struct {
	value string
}

func NewSlug(raw string) (Slug, error) {
	if raw == "" {
		return Slug{}, ErrSlugEmpty
	}
	if len(raw) < minSlugLen {
		return Slug{}, fmt.Errorf("categories: length %d below minimum %d: %w", len(raw), minSlugLen, ErrSlugTooShort)
	}
	if len(raw) > maxSlugLen {
		return Slug{}, fmt.Errorf("categories: length %d exceeds %d: %w", len(raw), maxSlugLen, ErrSlugTooLong)
	}
	if raw[0] == '-' || raw[len(raw)-1] == '-' {
		return Slug{}, ErrSlugEdgeHyphen
	}

	var prevHyphen bool
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c >= 'a' && c <= 'z':
			prevHyphen = false
		case c >= '0' && c <= '9':
			prevHyphen = false
		case c == '-':
			if prevHyphen {
				return Slug{}, ErrSlugDoubleHyphen
			}
			prevHyphen = true
		default:
			return Slug{}, fmt.Errorf("categories: %q: %w", raw, ErrSlugInvalidChars)
		}
	}

	return Slug{value: raw}, nil
}

func (s Slug) String() string {
	return s.value
}

func (s Slug) Equal(other Slug) bool {
	return s.value == other.value
}
