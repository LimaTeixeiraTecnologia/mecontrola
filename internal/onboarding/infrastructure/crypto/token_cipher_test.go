package crypto_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	onboardingcrypto "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/crypto"
)

type TokenCipherSuite struct {
	suite.Suite
}

func TestTokenCipherSuite(t *testing.T) {
	suite.Run(t, new(TokenCipherSuite))
}

func (s *TokenCipherSuite) SetupTest() {}

func (s *TokenCipherSuite) TestNewTokenCipher() {
	type args struct {
		key string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(*onboardingcrypto.TokenCipher, error)
	}{
		{
			name: "deve criptografar e descriptografar token",
			args: args{key: "12345678901234567890123456789012"},
			expect: func(cipher *onboardingcrypto.TokenCipher, err error) {
				s.Require().NoError(err)

				encryptedToken, err := cipher.Encrypt(context.Background(), "activation-token")
				s.Require().NoError(err)
				s.NotContains(encryptedToken, "activation-token")

				decryptedToken, err := cipher.Decrypt(context.Background(), encryptedToken)
				s.Require().NoError(err)
				s.Equal("activation-token", decryptedToken)
			},
		},
		{
			name: "deve rejeitar chave invalida",
			args: args{key: "short"},
			expect: func(cipher *onboardingcrypto.TokenCipher, err error) {
				s.Nil(cipher)
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			cipher, err := onboardingcrypto.NewTokenCipher(scenario.args.key)
			scenario.expect(cipher, err)
		})
	}
}
