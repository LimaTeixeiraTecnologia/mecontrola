package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type TokenHashPrefixSuite struct {
	suite.Suite
}

func TestTokenHashPrefixSuite(t *testing.T) {
	suite.Run(t, new(TokenHashPrefixSuite))
}

func (s *TokenHashPrefixSuite) TestTokenHashPrefix() {
	scenarios := []struct {
		name     string
		hash     []byte
		expected string
	}{
		{name: "hash vazio retorna string vazia", hash: nil, expected: ""},
		{name: "hash menor que 4 bytes mantem todo o hex", hash: []byte{0xab, 0xcd}, expected: "abcd"},
		{name: "hash de exatamente 4 bytes retorna 8 chars", hash: []byte{0xde, 0xad, 0xbe, 0xef}, expected: "deadbeef"},
		{name: "hash maior trunca para 8 chars", hash: []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}, expected: "01234567"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expected, valueobjects.TokenHashPrefix(scenario.hash))
		})
	}
}
