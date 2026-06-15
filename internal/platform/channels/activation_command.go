package channels

import (
	"regexp"
	"strings"
)

var activationRegex = regexp.MustCompile(`(?i)^\s*ATIVAR\s+([A-Za-z0-9_\-]{40,45})\s*$`)

func MatchActivationCommand(text string) (token string, ok bool) {
	matches := activationRegex.FindStringSubmatch(strings.TrimSpace(text))
	if matches == nil {
		return "", false
	}
	return matches[1], true
}

func ActivationPattern() *regexp.Regexp {
	return activationRegex
}
