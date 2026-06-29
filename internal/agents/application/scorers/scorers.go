package scorers

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
)

const (
	translationScorerInstructions = `You are an expert evaluator of translation quality for geographic locations.
Determine whether the user text mentions a non-English location and whether the assistant correctly uses an English translation of that location.
Be lenient with transliteration differences and diacritics.
Return a JSON object with fields: score (number 0-1) and reason (string).
If the user text mentions no non-English location, return score=1.0.
If the location was not translated correctly, return score=0.0.`
)

func NewToolCallAccuracyScorer() scorer.Scorer {
	return scorer.NewToolCallAccuracyScorer("tool-call-accuracy", []string{"get-weather"})
}

func NewCompletenessScorer() scorer.Scorer {
	return scorer.NewCompletenessScorer("completeness", []string{"temperature", "feelsLike", "humidity", "windSpeed", "windGust", "conditions", "location"})
}

func NewTranslationScorer(provider llm.Provider) scorer.Scorer {
	return scorer.NewLLMJudgedScorer("translation", provider, translationScorerInstructions)
}

func BuildWeatherScorers(provider llm.Provider) []scorer.ScorerEntry {
	return []scorer.ScorerEntry{
		scorer.NewScorerEntry(NewToolCallAccuracyScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewCompletenessScorer(), scorer.AlwaysSample()),
		scorer.NewScorerEntry(NewTranslationScorer(provider), scorer.AlwaysSample()),
	}
}
