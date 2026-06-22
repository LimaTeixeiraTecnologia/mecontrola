package valueobjects

const (
	scoreSignalWeight     = 0.45
	scoreConfidenceWeight = 0.30
	scoreQualityWeight    = 0.25

	signalPrecedenceMax = 5

	ScoreAutoThreshold    = 0.80
	ScoreConfirmThreshold = 0.55

	scoreAmbiguityDelta = 0.10
	tokenScoreCeiling   = 0.79
	fuzzyScoreCeiling   = 0.70
)

type MatchScore float64

func NewMatchScore(signal SignalType, confidence Confidence, quality MatchQuality) MatchScore {
	signalNorm := float64(signal.Precedence()) / float64(signalPrecedenceMax)
	raw := scoreSignalWeight*signalNorm +
		scoreConfidenceWeight*confidence.Weight() +
		scoreQualityWeight*quality.Weight()

	score := clampScore(MatchScore(raw))
	switch quality {
	case MatchQualityToken:
		if score > MatchScore(tokenScoreCeiling) {
			return MatchScore(tokenScoreCeiling)
		}
	case MatchQualityFuzzy:
		if score > MatchScore(fuzzyScoreCeiling) {
			return MatchScore(fuzzyScoreCeiling)
		}
	}
	return score
}

func clampScore(s MatchScore) MatchScore {
	switch {
	case s < 0:
		return 0
	case s > 1:
		return 1
	default:
		return s
	}
}

func (s MatchScore) Float64() float64 {
	return float64(s)
}

func (s MatchScore) IsAuto() bool {
	return s >= ScoreAutoThreshold
}

func (s MatchScore) NeedsConfirmation() bool {
	return s >= ScoreConfirmThreshold && s < ScoreAutoThreshold
}

func ClassifyByScore(scores []MatchScore) SearchOutcome {
	if len(scores) == 0 {
		return SearchOutcomeNoMatch
	}

	top := scores[0]
	if top < ScoreConfirmThreshold {
		return SearchOutcomeNoMatch
	}

	if len(scores) > 1 && top-scores[1] < scoreAmbiguityDelta {
		return SearchOutcomeAmbiguous
	}

	if top < ScoreAutoThreshold {
		return SearchOutcomeAmbiguous
	}

	return SearchOutcomeMatched
}
