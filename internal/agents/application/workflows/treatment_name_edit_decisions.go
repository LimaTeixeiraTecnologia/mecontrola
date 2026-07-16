package workflows

import (
	"strings"
	"time"
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
	return trimmed, true
}

func DecideTreatmentNameEditExpiry(state TreatmentNameEditState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > treatmentNameEditTTL
}
