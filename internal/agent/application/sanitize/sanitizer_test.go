package sanitize_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/sanitize"
)

type SanitizerSuite struct {
	suite.Suite
	s *sanitize.Sanitizer
}

func TestSanitizerSuite(t *testing.T) {
	suite.Run(t, new(SanitizerSuite))
}

func (s *SanitizerSuite) SetupTest() {
	sanitizer, err := sanitize.NewSanitizer(50)
	s.Require().NoError(err)
	s.s = sanitizer
}

func (s *SanitizerSuite) TestEmptyAfterTrim() {
	_, err := s.s.Clean("   \n\t  ")
	s.Require().ErrorIs(err, sanitize.ErrEmpty)
}

func (s *SanitizerSuite) TestPreservesLegitMessage() {
	out, err := s.s.Clean("  gastei 35,90 no mercado  ")
	s.Require().NoError(err)
	s.Equal("gastei 35,90 no mercado", out)
}

func (s *SanitizerSuite) TestKeepsShortAmountsUnmasked() {
	out, err := s.s.Clean("recebi 350000 de salario")
	s.Require().NoError(err)
	s.Equal("recebi 350000 de salario", out)
}

func (s *SanitizerSuite) TestMasksFormattedCPF() {
	out, err := s.s.Clean("meu cpf 123.456.789-09 ok")
	s.Require().NoError(err)
	s.Contains(out, "[REDACTED_CPF]")
	s.NotContains(out, "123.456.789-09")
}

func (s *SanitizerSuite) TestMasksCardLikeSequences() {
	cases := []string{
		"cartao 4111 1111 1111 1111 final",
		"cartao 4111-1111-1111-1111 final",
		"cartao 4111111111111111 final",
	}
	for _, in := range cases {
		s.Run(in, func() {
			out, err := s.s.Clean(in)
			s.Require().NoError(err)
			s.Contains(out, "[REDACTED_CARD]")
			s.NotContains(out, "1111 1111")
		})
	}
}

func (s *SanitizerSuite) TestDoesNotMaskBrazilianPhoneLength() {
	out, err := s.s.Clean("zap 11987654321 agora")
	s.Require().NoError(err)
	s.NotContains(out, "[REDACTED_CARD]")
	s.NotContains(out, "[REDACTED_CPF]")
}

func (s *SanitizerSuite) TestNormalizesControlAndWhitespace() {
	out, err := s.s.Clean("linha1\n\nlinha2\tfim")
	s.Require().NoError(err)
	s.Equal("linha1 linha2 fim", out)
}

func (s *SanitizerSuite) TestCapsLength() {
	out, err := s.s.Clean(strings.Repeat("a", 200))
	s.Require().NoError(err)
	s.LessOrEqual(len([]rune(out)), 50)
}

func (s *SanitizerSuite) TestDefaultMaxWhenNonPositive() {
	sanitizer, err := sanitize.NewSanitizer(0)
	s.Require().NoError(err)
	out, err := sanitizer.Clean("ola")
	s.Require().NoError(err)
	s.Equal("ola", out)
}
