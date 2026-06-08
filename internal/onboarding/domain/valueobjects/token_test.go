package valueobjects_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type TokenSuite struct {
	suite.Suite
}

func TestTokenSuite(t *testing.T) {
	suite.Run(t, new(TokenSuite))
}

func (s *TokenSuite) TestNewToken_GeneratesUniqueTokens() {
	t1, err := valueobjects.NewToken()
	s.Require().NoError(err)

	t2, err := valueobjects.NewToken()
	s.Require().NoError(err)

	s.NotEqual(t1.ClearText(), t2.ClearText())
}

func (s *TokenSuite) TestNewToken_ClearTextIs43Chars() {
	t, err := valueobjects.NewToken()
	s.Require().NoError(err)
	s.Len(t.ClearText(), 43)
}

func (s *TokenSuite) TestNewToken_ClearTextURLSafe() {
	t, err := valueobjects.NewToken()
	s.Require().NoError(err)
	clear := t.ClearText()
	s.False(strings.Contains(clear, "+"), "clear text should not contain +")
	s.False(strings.Contains(clear, "/"), "clear text should not contain /")
	s.False(strings.Contains(clear, "="), "clear text should not have padding")
}

func (s *TokenSuite) TestNewToken_HashIs32Bytes() {
	t, err := valueobjects.NewToken()
	s.Require().NoError(err)
	s.Len(t.Hash(), 32)
}

func (s *TokenSuite) TestNewToken_HashIsDeterministic() {
	t, err := valueobjects.NewToken()
	s.Require().NoError(err)

	recovered, err := valueobjects.TokenFromClear(t.ClearText())
	s.Require().NoError(err)

	s.Equal(t.HashHex(), recovered.HashHex())
}

func (s *TokenSuite) TestNewToken_StringIsRedacted() {
	t, err := valueobjects.NewToken()
	s.Require().NoError(err)
	s.Equal("[REDACTED]", t.String())
	s.NotContains(t.String(), t.ClearText())
}

func (s *TokenSuite) TestTokenFromClear_EmptyReturnsError() {
	_, err := valueobjects.TokenFromClear("")
	s.ErrorIs(err, valueobjects.ErrTokenEmpty)
}

func (s *TokenSuite) TestTokenFromClear_InvalidBase64ReturnsError() {
	_, err := valueobjects.TokenFromClear("not-valid-base64!!")
	s.ErrorIs(err, valueobjects.ErrTokenInvalid)
}

func (s *TokenSuite) TestHashPrefix_Returns8HexChars() {
	t, err := valueobjects.NewToken()
	s.Require().NoError(err)
	s.Len(t.HashPrefix(), 8)
}
