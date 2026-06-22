package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type MatchScoreSuite struct {
	suite.Suite
}

func TestMatchScoreSuite(t *testing.T) {
	suite.Run(t, new(MatchScoreSuite))
}

func (s *MatchScoreSuite) TestExactCanonicalHighIsPerfect() {
	score := valueobjects.NewMatchScore(valueobjects.SignalTypeCanonicalName, valueobjects.ConfidenceHigh, valueobjects.MatchQualityExact)
	s.InDelta(1.0, score.Float64(), 1e-9)
	s.True(score.IsAuto())
	s.False(score.NeedsConfirmation())
}

func (s *MatchScoreSuite) TestScoreStaysInUnitRange() {
	signals := []valueobjects.SignalType{
		valueobjects.SignalTypeCanonicalName,
		valueobjects.SignalTypeAlias,
		valueobjects.SignalTypePhrase,
		valueobjects.SignalTypeMerchant,
		valueobjects.SignalTypeSegment,
	}
	confidences := []valueobjects.Confidence{valueobjects.ConfidenceHigh, valueobjects.ConfidenceMedium, valueobjects.ConfidenceLow}
	qualities := []valueobjects.MatchQuality{valueobjects.MatchQualityExact, valueobjects.MatchQualityToken, valueobjects.MatchQualityFuzzy}

	for _, sig := range signals {
		for _, conf := range confidences {
			for _, qual := range qualities {
				score := valueobjects.NewMatchScore(sig, conf, qual)
				s.GreaterOrEqual(score.Float64(), 0.0)
				s.LessOrEqual(score.Float64(), 1.0)
			}
		}
	}
}

func (s *MatchScoreSuite) TestQualityIsMonotonicAtEqualSignalAndConfidence() {
	exact := valueobjects.NewMatchScore(valueobjects.SignalTypeAlias, valueobjects.ConfidenceHigh, valueobjects.MatchQualityExact)
	token := valueobjects.NewMatchScore(valueobjects.SignalTypeAlias, valueobjects.ConfidenceHigh, valueobjects.MatchQualityToken)
	fuzzy := valueobjects.NewMatchScore(valueobjects.SignalTypeAlias, valueobjects.ConfidenceHigh, valueobjects.MatchQualityFuzzy)
	s.Greater(exact.Float64(), token.Float64())
	s.Greater(token.Float64(), fuzzy.Float64())
}

func (s *MatchScoreSuite) TestFuzzyNeverReachesAutoThreshold() {
	score := valueobjects.NewMatchScore(valueobjects.SignalTypeCanonicalName, valueobjects.ConfidenceHigh, valueobjects.MatchQualityFuzzy)
	s.False(score.IsAuto())
	s.Less(score.Float64(), valueobjects.ScoreAutoThreshold)
	s.GreaterOrEqual(score.Float64(), valueobjects.ScoreConfirmThreshold)
}

func (s *MatchScoreSuite) TestOnlyExactCanAutoLog() {
	signals := []valueobjects.SignalType{
		valueobjects.SignalTypeCanonicalName,
		valueobjects.SignalTypeAlias,
		valueobjects.SignalTypePhrase,
		valueobjects.SignalTypeMerchant,
		valueobjects.SignalTypeSegment,
	}
	confidences := []valueobjects.Confidence{valueobjects.ConfidenceHigh, valueobjects.ConfidenceMedium, valueobjects.ConfidenceLow}

	for _, sig := range signals {
		for _, conf := range confidences {
			token := valueobjects.NewMatchScore(sig, conf, valueobjects.MatchQualityToken)
			fuzzy := valueobjects.NewMatchScore(sig, conf, valueobjects.MatchQualityFuzzy)
			s.Falsef(token.IsAuto(), "token nunca pode auto-logar (signal=%s conf=%s)", sig, conf)
			s.Falsef(fuzzy.IsAuto(), "fuzzy nunca pode auto-logar (signal=%s conf=%s)", sig, conf)
		}
	}

	exact := valueobjects.NewMatchScore(valueobjects.SignalTypeCanonicalName, valueobjects.ConfidenceHigh, valueobjects.MatchQualityExact)
	s.True(exact.IsAuto())
}

func (s *MatchScoreSuite) TestScoreLatticeIsTotalAndOnlyExactAutoLogs() {
	signals := []valueobjects.SignalType{
		valueobjects.SignalTypeCanonicalName,
		valueobjects.SignalTypeAlias,
		valueobjects.SignalTypePhrase,
		valueobjects.SignalTypeMerchant,
		valueobjects.SignalTypeSegment,
	}
	confidences := []valueobjects.Confidence{valueobjects.ConfidenceHigh, valueobjects.ConfidenceMedium, valueobjects.ConfidenceLow}
	qualities := []valueobjects.MatchQuality{valueobjects.MatchQualityExact, valueobjects.MatchQualityToken, valueobjects.MatchQualityFuzzy}

	bucket := func(score valueobjects.MatchScore) string {
		switch {
		case score >= valueobjects.ScoreAutoThreshold:
			return "auto"
		case score >= valueobjects.ScoreConfirmThreshold:
			return "confirm"
		default:
			return "reject"
		}
	}

	combos := 0
	autoCount := 0
	for _, sig := range signals {
		for _, conf := range confidences {
			for _, qual := range qualities {
				combos++
				b := bucket(valueobjects.NewMatchScore(sig, conf, qual))
				if b == "auto" {
					autoCount++
					s.Equalf(valueobjects.MatchQualityExact, qual, "auto-log só pode vir de match exato (signal=%s conf=%s qual=%s)", sig, conf, qual)
				}
			}
		}
	}

	s.Equal(45, combos, "domínio finito: 5 signal x 3 confidence x 3 quality")
	s.Equal(5, autoCount, "exatamente 5 combinações exatas auto-logam; treliça total e verificada")
}

func (s *MatchScoreSuite) TestClassifyByScore() {
	scenarios := []struct {
		name   string
		scores []valueobjects.MatchScore
		want   valueobjects.SearchOutcome
	}{
		{name: "vazio eh no_match", scores: nil, want: valueobjects.SearchOutcomeNoMatch},
		{name: "topo abaixo de confirm eh no_match", scores: []valueobjects.MatchScore{0.40}, want: valueobjects.SearchOutcomeNoMatch},
		{name: "topo alto e unico eh matched", scores: []valueobjects.MatchScore{0.95}, want: valueobjects.SearchOutcomeMatched},
		{name: "topo na faixa de confirm eh ambiguous", scores: []valueobjects.MatchScore{0.62}, want: valueobjects.SearchOutcomeAmbiguous},
		{name: "dois topos proximos eh ambiguous", scores: []valueobjects.MatchScore{0.95, 0.90}, want: valueobjects.SearchOutcomeAmbiguous},
		{name: "topo alto e segundo distante eh matched", scores: []valueobjects.MatchScore{0.95, 0.60}, want: valueobjects.SearchOutcomeMatched},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.want, valueobjects.ClassifyByScore(scenario.scores))
		})
	}
}
