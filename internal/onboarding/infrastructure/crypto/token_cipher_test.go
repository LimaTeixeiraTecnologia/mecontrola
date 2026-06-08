package crypto_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	onboardingcrypto "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/crypto"
)

func TestTokenCipher_RoundTrip(t *testing.T) {
	cipher, err := onboardingcrypto.NewTokenCipher("12345678901234567890123456789012")
	require.NoError(t, err)

	encrypted, err := cipher.Encrypt(context.Background(), "activation-token")
	require.NoError(t, err)
	require.NotContains(t, encrypted, "activation-token")

	decrypted, err := cipher.Decrypt(context.Background(), encrypted)
	require.NoError(t, err)
	require.Equal(t, "activation-token", decrypted)
}

func TestTokenCipher_RejectsInvalidKey(t *testing.T) {
	_, err := onboardingcrypto.NewTokenCipher("short")
	require.Error(t, err)
}
