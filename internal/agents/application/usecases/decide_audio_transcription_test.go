package usecases

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
)

type DecideAudioTranscriptionSuite struct {
	suite.Suite
}

func TestDecideAudioTranscriptionSuite(t *testing.T) {
	suite.Run(t, new(DecideAudioTranscriptionSuite))
}

func (s *DecideAudioTranscriptionSuite) SetupTest() {}

func (s *DecideAudioTranscriptionSuite) TestDecideAudioTranscription() {
	confidenceLow := 0.5
	confidenceHigh := 0.95

	scenarios := []struct {
		name   string
		args   input.AudioDecisionInput
		expect func(decision AudioDecision)
	}{
		{
			name: "deve aprovar transcricao pt-br coerente e confiavel",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "  gastei   50 reais\tno mercado  ",
				Language:      "pt",
				MinConfidence: 0.80,
				Confidence:    &confidenceHigh,
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeApproved, decision.Outcome)
				s.Equal(AudioReasonApproved, decision.Reason)
				s.Equal("gastei 50 reais no mercado", decision.CanonicalText)
			},
		},
		{
			name: "deve falhar tecnicamente quando stt retorna erro",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				MinConfidence: 0.80,
				STTError:      errors.New("upstream timeout"),
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeTranscriptionFailed, decision.Outcome)
				s.Equal(AudioReasonSTTError, decision.Reason)
				s.Empty(decision.CanonicalText)
			},
		},
		{
			name: "deve marcar incerto quando truncado mesmo com texto valido",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "gastei 50 reais no mercado",
				Language:      "pt",
				Truncated:     true,
				MinConfidence: 0.80,
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeTranscriptionUncertain, decision.Outcome)
				s.Equal(AudioReasonTruncated, decision.Reason)
				s.Empty(decision.CanonicalText)
			},
		},
		{
			name: "deve marcar incerto quando texto vazio apos normalizacao tecnica",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "   \n\t  ",
				Language:      "pt",
				MinConfidence: 0.80,
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeTranscriptionUncertain, decision.Outcome)
				s.Equal(AudioReasonEmptyText, decision.Reason)
				s.Empty(decision.CanonicalText)
			},
		},
		{
			name: "deve marcar incerto quando texto e apenas pontuacao",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "... ??",
				Language:      "pt",
				MinConfidence: 0.80,
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeTranscriptionUncertain, decision.Outcome)
				s.Equal(AudioReasonIncoherent, decision.Reason)
				s.Empty(decision.CanonicalText)
			},
		},
		{
			name: "deve marcar incerto quando texto e repeticao ininteligivel",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "aaaaaaaaaaaaaaaaaaaaaaa",
				Language:      "pt",
				MinConfidence: 0.80,
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeTranscriptionUncertain, decision.Outcome)
				s.Equal(AudioReasonIncoherent, decision.Reason)
				s.Empty(decision.CanonicalText)
			},
		},
		{
			name: "deve marcar incerto quando idioma nao e pt-br",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "I spent 50 dollars on groceries",
				Language:      "en",
				MinConfidence: 0.80,
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeTranscriptionUncertain, decision.Outcome)
				s.Equal(AudioReasonLanguageUnsupported, decision.Reason)
				s.Empty(decision.CanonicalText)
			},
		},
		{
			name: "deve marcar incerto quando idioma esta ausente",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "gastei 50 reais no mercado",
				Language:      "",
				MinConfidence: 0.80,
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeTranscriptionUncertain, decision.Outcome)
				s.Equal(AudioReasonLanguageUnsupported, decision.Reason)
				s.Empty(decision.CanonicalText)
			},
		},
		{
			name: "deve marcar incerto quando confianca abaixo do minimo configurado",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "gastei 50 reais no mercado",
				Language:      "pt",
				MinConfidence: 0.80,
				Confidence:    &confidenceLow,
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeTranscriptionUncertain, decision.Outcome)
				s.Equal(AudioReasonLowConfidence, decision.Reason)
				s.Empty(decision.CanonicalText)
			},
		},
		{
			name: "deve aprovar quando confianca ausente e demais sinais tecnicos ok",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "gastei 50 reais no mercado",
				Language:      "pt-BR",
				MinConfidence: 0.80,
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeApproved, decision.Outcome)
				s.Equal(AudioReasonApproved, decision.Reason)
				s.Equal("gastei 50 reais no mercado", decision.CanonicalText)
			},
		},
		{
			name: "deve aprovar quando confianca e exatamente igual ao minimo configurado",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "gastei 50 reais no mercado",
				Language:      "pt",
				MinConfidence: 0.80,
				Confidence:    func() *float64 { v := 0.80; return &v }(),
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeApproved, decision.Outcome)
				s.Equal(AudioReasonApproved, decision.Reason)
			},
		},
		{
			name: "ambiguidade financeira com transcricao tecnicamente confiavel nao vira incerteza tecnica",
			args: input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          "quero registrar uma despesa",
				Language:      "pt",
				MinConfidence: 0.80,
			},
			expect: func(decision AudioDecision) {
				s.Equal(AudioOutcomeApproved, decision.Outcome)
				s.Equal(AudioReasonApproved, decision.Reason)
				s.Equal("quero registrar uma despesa", decision.CanonicalText)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			decision := DecideAudioTranscription(scenario.args)
			scenario.expect(decision)
		})
	}
}

func (s *DecideAudioTranscriptionSuite) TestOnlyApprovedOutcomeAllowsHandleInbound() {
	scenarios := []struct {
		outcome AudioOutcome
		allowed bool
	}{
		{AudioOutcomeApproved, true},
		{AudioOutcomeRejected, false},
		{AudioOutcomeTranscriptionUncertain, false},
		{AudioOutcomeTranscriptionFailed, false},
		{AudioOutcomeDispatched, false},
	}

	for _, scenario := range scenarios {
		s.Run(string(scenario.outcome), func() {
			s.Equal(scenario.allowed, scenario.outcome.AllowsHandleInbound())
		})
	}
}

func (s *DecideAudioTranscriptionSuite) TestAudioOutcomeIsValid() {
	s.True(AudioOutcomeApproved.IsValid())
	s.True(AudioOutcomeRejected.IsValid())
	s.True(AudioOutcomeTranscriptionUncertain.IsValid())
	s.True(AudioOutcomeTranscriptionFailed.IsValid())
	s.True(AudioOutcomeDispatched.IsValid())
	s.False(AudioOutcome("unknown").IsValid())
	s.False(AudioOutcome("").IsValid())
}

func (s *DecideAudioTranscriptionSuite) TestAudioReasonIsValid() {
	s.True(AudioReasonApproved.IsValid())
	s.True(AudioReasonSTTError.IsValid())
	s.True(AudioReasonTruncated.IsValid())
	s.True(AudioReasonEmptyText.IsValid())
	s.True(AudioReasonIncoherent.IsValid())
	s.True(AudioReasonLanguageUnsupported.IsValid())
	s.True(AudioReasonLowConfidence.IsValid())
	s.True(AudioReasonInvalidPayload.IsValid())
	s.True(AudioReasonMediaUnavailable.IsValid())
	s.True(AudioReasonSizeExceeded.IsValid())
	s.True(AudioReasonDurationUnavailable.IsValid())
	s.True(AudioReasonDurationExceeded.IsValid())
	s.True(AudioReasonCostExceeded.IsValid())
	s.False(AudioReason("unknown").IsValid())
	s.False(AudioReason("").IsValid())
}

func (s *DecideAudioTranscriptionSuite) TestNormalizationIsTechnicalOnlyWithoutSemanticEnrichment() {
	decision := DecideAudioTranscription(input.AudioDecisionInput{
		Model:         "openai/whisper-large-v3",
		Text:          "gastei   50   reais  no  mercado\n\n",
		Language:      "pt",
		MinConfidence: 0.80,
	})
	s.Equal(AudioOutcomeApproved, decision.Outcome)
	s.Equal("gastei 50 reais no mercado", decision.CanonicalText)
}

func (s *DecideAudioTranscriptionSuite) TestZeroValueInputProducesEmptyTextUncertain() {
	decision := DecideAudioTranscription(input.AudioDecisionInput{})
	s.Equal(AudioOutcomeTranscriptionUncertain, decision.Outcome)
	s.Equal(AudioReasonEmptyText, decision.Reason)
	s.Empty(decision.CanonicalText)
}
