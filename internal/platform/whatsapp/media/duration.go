package media

import (
	"strings"
	"time"
)

func DetermineDuration(mimeType string, data []byte) (time.Duration, error) {
	switch normalizeMimeType(mimeType) {
	case "audio/ogg", "audio/opus":
		return oggOpusDuration(data)
	case "audio/mp4", "audio/m4a", "audio/x-m4a", "audio/aac", "audio/mp4a-latm":
		return m4aAACDuration(data)
	default:
		return 0, ErrUnsupportedMimeType
	}
}

func CheckMaxDuration(duration, max time.Duration) error {
	if max > 0 && duration > max {
		return ErrAudioDurationExceeded
	}
	return nil
}

func normalizeMimeType(mimeType string) string {
	base, _, _ := strings.Cut(mimeType, ";")
	return strings.ToLower(strings.TrimSpace(base))
}
