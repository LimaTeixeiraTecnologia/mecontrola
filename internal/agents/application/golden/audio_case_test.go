package golden

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
)

const audioGoldenMinCasesPerGroup = 3

type AudioCaseSuite struct {
	suite.Suite
}

func TestAudioCaseSuite(t *testing.T) {
	suite.Run(t, new(AudioCaseSuite))
}

func (s *AudioCaseSuite) TestAudioPairedCasesReferenceExistingTextualCases() {
	textualNames := map[string]bool{}
	for _, textCase := range AllCases() {
		textualNames[textCase.Name] = true
	}

	for _, c := range AudioPairedCases() {
		s.Truef(textualNames[c.TextCaseName],
			"RF-30: caso de audio %q referencia o caso textual %q, que nao existe no golden textual pre-existente; "+
				"pareamento nao pode inventar caso proprio", c.Name, c.TextCaseName)

		resolved, ok := c.ResolveTextCase()
		s.Require().Truef(ok, "caso textual %q nao pode ser resolvido", c.TextCaseName)
		s.Require().NoErrorf(resolved.Validate(), "caso textual pareado %q invalido", c.TextCaseName)
	}
}

func (s *AudioCaseSuite) TestAudioPairedCasesCoverAllRequiredGroups() {
	cases := AudioPairedCases()
	s.Require().NotEmpty(cases)

	byGroup := map[AudioGroup]int{}
	names := map[string]bool{}
	for _, c := range cases {
		s.Truef(c.Group.IsValid(), "grupo de audio invalido em %q: %q", c.Name, c.Group)
		s.NotEmptyf(c.Name, "caso de audio sem nome")
		s.Falsef(names[c.Name], "nome de caso de audio duplicado: %q", c.Name)
		names[c.Name] = true
		byGroup[c.Group]++
	}

	for _, group := range AllAudioGroups() {
		s.GreaterOrEqualf(byGroup[group], audioGoldenMinCasesPerGroup,
			"grupo %q deve ter ao menos %d casos positivos pareados", group, audioGoldenMinCasesPerGroup)
	}
}

var unmaskedPhonePattern = regexp.MustCompile(`\+55\d{6,}`)

func (s *AudioCaseSuite) TestAudioPairedCasesAreAnonymized() {
	forbidden := []string{"3140d64a", "cf8be1b10035", "@"}
	for _, c := range AudioPairedCases() {
		textCase, ok := c.ResolveTextCase()
		s.Require().True(ok)

		fields := []string{c.SpokenText, textCase.Input, textCase.Origin, textCase.ResponseDescribe}
		for _, turn := range textCase.PriorTurns {
			fields = append(fields, turn.UserMessage)
		}
		for _, field := range fields {
			for _, term := range forbidden {
				s.NotContainsf(field, term, "caso de audio %q não pode conter dado pessoal/verbatim (%s)", c.Name, term)
			}
			s.Falsef(unmaskedPhonePattern.MatchString(field),
				"caso de audio %q não pode conter telefone sem máscara; use o formato +55****NNNN", c.Name)
		}
	}
}

func (s *AudioCaseSuite) TestSpokenTextIsAnAudioVariantNotACopy() {
	identical := 0
	for _, c := range AudioPairedCases() {
		textCase, ok := c.ResolveTextCase()
		s.Require().True(ok)
		if normalizeSpoken(c.SpokenText) == normalizeSpoken(textCase.Input) {
			identical++
		}
	}
	s.Lessf(identical, len(AudioPairedCases()),
		"RF-30: se todo SpokenText for identico ao input textual, o pareamento nao exercita nenhuma "+
			"caracteristica real de transcricao (pontuacao ausente, caixa baixa, espacamento duplo)")
}

func normalizeSpoken(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func (s *AudioCaseSuite) TestAudioPairedCasesDecodeToApprovedCanonicalText() {
	for _, c := range AudioPairedCases() {
		s.Run(c.Name, func() {
			confidence := 0.95
			decision := usecases.DecideAudioTranscription(input.AudioDecisionInput{
				Model:         "openai/whisper-large-v3",
				Text:          c.SpokenText,
				Language:      "pt",
				Truncated:     false,
				Confidence:    &confidence,
				MinConfidence: 0.80,
			})

			s.Equal(usecases.AudioOutcomeApproved, decision.Outcome,
				"caso de audio %q deveria ser aprovado a partir do texto transcrito simulado", c.Name)
			s.True(decision.Outcome.AllowsHandleInbound(),
				"outcome aprovado deve permitir HandleInbound")

			expectedCanonical := strings.Join(strings.Fields(c.SpokenText), " ")
			s.Equal(expectedCanonical, decision.CanonicalText,
				"texto canonico deve ser o texto transcrito com espacos normalizados, sem reescrita semantica")
		})
	}
}
