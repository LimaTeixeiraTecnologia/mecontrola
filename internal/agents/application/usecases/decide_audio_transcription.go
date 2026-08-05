package usecases

import (
	"strings"
	"unicode"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
)

type AudioOutcome string

const (
	AudioOutcomeProcessing             AudioOutcome = "processing"
	AudioOutcomeApproved               AudioOutcome = "approved"
	AudioOutcomeRejected               AudioOutcome = "rejected"
	AudioOutcomeTranscriptionUncertain AudioOutcome = "transcription_uncertain"
	AudioOutcomeTranscriptionFailed    AudioOutcome = "transcription_failed"
	AudioOutcomeDispatched             AudioOutcome = "dispatched"
)

func (o AudioOutcome) String() string {
	return string(o)
}

func (o AudioOutcome) IsValid() bool {
	switch o {
	case AudioOutcomeProcessing,
		AudioOutcomeApproved,
		AudioOutcomeRejected,
		AudioOutcomeTranscriptionUncertain,
		AudioOutcomeTranscriptionFailed,
		AudioOutcomeDispatched:
		return true
	default:
		return false
	}
}

func (o AudioOutcome) AllowsHandleInbound() bool {
	return o == AudioOutcomeApproved
}

type AudioReason string

const (
	AudioReasonProcessing          AudioReason = "processing"
	AudioReasonInterrupted         AudioReason = "interrupted"
	AudioReasonApproved            AudioReason = "approved"
	AudioReasonSTTError            AudioReason = "stt_error"
	AudioReasonTruncated           AudioReason = "truncated"
	AudioReasonEmptyText           AudioReason = "empty_text"
	AudioReasonIncoherent          AudioReason = "incoherent"
	AudioReasonLanguageUnsupported AudioReason = "language_unsupported"
	AudioReasonLowConfidence       AudioReason = "low_confidence"
	AudioReasonInvalidPayload      AudioReason = "invalid_payload"
	AudioReasonMediaUnavailable    AudioReason = "media_unavailable"
	AudioReasonSizeExceeded        AudioReason = "size_exceeded"
	AudioReasonDurationUnavailable AudioReason = "duration_unavailable"
	AudioReasonDurationExceeded    AudioReason = "duration_exceeded"
	AudioReasonCostExceeded        AudioReason = "cost_exceeded"
)

func (r AudioReason) String() string {
	return string(r)
}

func (r AudioReason) IsValid() bool {
	switch r {
	case AudioReasonProcessing,
		AudioReasonInterrupted,
		AudioReasonApproved,
		AudioReasonSTTError,
		AudioReasonTruncated,
		AudioReasonEmptyText,
		AudioReasonIncoherent,
		AudioReasonLanguageUnsupported,
		AudioReasonLowConfidence,
		AudioReasonInvalidPayload,
		AudioReasonMediaUnavailable,
		AudioReasonSizeExceeded,
		AudioReasonDurationUnavailable,
		AudioReasonDurationExceeded,
		AudioReasonCostExceeded:
		return true
	default:
		return false
	}
}

type AudioDecision struct {
	Outcome       AudioOutcome
	CanonicalText string
	Reason        AudioReason
}

const minAudioAlnumRunes = 3

func DecideAudioTranscription(in input.AudioDecisionInput) AudioDecision {
	if in.STTError != nil {
		return AudioDecision{Outcome: AudioOutcomeTranscriptionFailed, Reason: AudioReasonSTTError}
	}
	if in.Truncated {
		return AudioDecision{Outcome: AudioOutcomeTranscriptionUncertain, Reason: AudioReasonTruncated}
	}

	normalized := normalizeAudioText(in.Text)
	if normalized == "" {
		return AudioDecision{Outcome: AudioOutcomeTranscriptionUncertain, Reason: AudioReasonEmptyText}
	}
	if isIncoherentAudioText(normalized) {
		return AudioDecision{Outcome: AudioOutcomeTranscriptionUncertain, Reason: AudioReasonIncoherent}
	}
	if !isPortugueseAudioLanguage(in.Language) {
		return AudioDecision{Outcome: AudioOutcomeTranscriptionUncertain, Reason: AudioReasonLanguageUnsupported}
	}
	if in.Confidence != nil && *in.Confidence < in.MinConfidence {
		return AudioDecision{Outcome: AudioOutcomeTranscriptionUncertain, Reason: AudioReasonLowConfidence}
	}

	return AudioDecision{
		Outcome:       AudioOutcomeApproved,
		CanonicalText: normalized,
		Reason:        AudioReasonApproved,
	}
}

func normalizeAudioText(text string) string {
	var b strings.Builder
	lastWasSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if lastWasSpace {
				continue
			}
			lastWasSpace = true
			b.WriteRune(' ')
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		lastWasSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func alnumAudioRuneCount(text string) int {
	count := 0
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			count++
		}
	}
	return count
}

func isIncoherentAudioText(normalized string) bool {
	if alnumAudioRuneCount(normalized) < minAudioAlnumRunes {
		return true
	}
	return isRepeatedSingleAudioRune(normalized)
}

func isRepeatedSingleAudioRune(normalized string) bool {
	unique := map[rune]struct{}{}
	for _, r := range normalized {
		if unicode.IsSpace(r) {
			continue
		}
		unique[r] = struct{}{}
	}
	return len(unique) <= 1
}

func isPortugueseAudioLanguage(language string) bool {
	normalized := strings.ToLower(strings.TrimSpace(language))
	if normalized == "" {
		return false
	}
	return normalized == "pt" || strings.HasPrefix(normalized, "pt-")
}
