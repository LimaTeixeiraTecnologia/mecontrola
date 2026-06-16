package channels

import (
	"regexp"
	"strings"
)

var (
	activationRegex           = regexp.MustCompile(`(?i)^\s*ATIVAR\s+([A-Za-z0-9_\-]{40,45})\s*$`)
	activationSlashStartRegex = regexp.MustCompile(`(?i)^\s*/start\s+ATIVAR_([A-Za-z0-9_\-]{40,45})\s*$`)
)

func MatchActivationCommand(text string) (token string, ok bool) {
	trimmed := strings.TrimSpace(text)
	if token, ok := matchSpace(trimmed); ok {
		return token, true
	}
	if token, ok := matchSlashStart(trimmed); ok {
		return token, true
	}
	return "", false
}

func matchSpace(text string) (string, bool) {
	matches := activationRegex.FindStringSubmatch(text)
	if matches == nil {
		return "", false
	}
	return matches[1], true
}

func matchSlashStart(text string) (string, bool) {
	matches := activationSlashStartRegex.FindStringSubmatch(text)
	if matches == nil {
		return "", false
	}
	return matches[1], true
}

func ActivationPattern() *regexp.Regexp {
	return activationRegex
}
