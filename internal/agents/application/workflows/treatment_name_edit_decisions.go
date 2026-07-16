package workflows

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	treatmentNameMaxLen  = 40
	treatmentNameEditTTL = 15 * time.Minute
)

func DecideTreatmentName(hasName bool, raw string) (string, bool) {
	if !hasName {
		return "", false
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > treatmentNameMaxLen {
		return "", false
	}
	if !treatmentNameHasMeaningfulRune(trimmed) {
		return "", false
	}
	return trimmed, true
}

func DecideTreatmentNameTooLong(hasName bool, raw string) bool {
	if !hasName {
		return false
	}
	trimmed := strings.TrimSpace(raw)
	return utf8.RuneCountInString(trimmed) > treatmentNameMaxLen
}

func treatmentNameHasMeaningfulRune(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r > unicode.MaxASCII {
			return true
		}
	}
	return false
}

func DecideTreatmentNameEditExpiry(state TreatmentNameEditState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > treatmentNameEditTTL
}
