package valueobjects

import (
	"errors"
	"fmt"
)

var ErrInvalidCategoryDecisionSource = errors.New("transactions: category decision source invalido")

type CategoryDecisionSource uint8

const (
	CategoryDecisionSourceAutoMatched CategoryDecisionSource = iota + 1
	CategoryDecisionSourceUserSelectedCandidate
	CategoryDecisionSourceManualCanonicalID
	CategoryDecisionSourceSystemMigration
)

func ParseCategoryDecisionSource(s string) (CategoryDecisionSource, error) {
	switch s {
	case "auto_matched":
		return CategoryDecisionSourceAutoMatched, nil
	case "user_selected_candidate":
		return CategoryDecisionSourceUserSelectedCandidate, nil
	case "manual_canonical_id":
		return CategoryDecisionSourceManualCanonicalID, nil
	case "system_migration":
		return CategoryDecisionSourceSystemMigration, nil
	default:
		return 0, fmt.Errorf("transactions: %q: %w", s, ErrInvalidCategoryDecisionSource)
	}
}

func (s CategoryDecisionSource) String() string {
	switch s {
	case CategoryDecisionSourceAutoMatched:
		return "auto_matched"
	case CategoryDecisionSourceUserSelectedCandidate:
		return "user_selected_candidate"
	case CategoryDecisionSourceManualCanonicalID:
		return "manual_canonical_id"
	case CategoryDecisionSourceSystemMigration:
		return "system_migration"
	default:
		return ""
	}
}

func (s CategoryDecisionSource) IsValid() bool {
	return s >= CategoryDecisionSourceAutoMatched && s <= CategoryDecisionSourceSystemMigration
}

func (s CategoryDecisionSource) IsManual() bool {
	return s == CategoryDecisionSourceManualCanonicalID
}
