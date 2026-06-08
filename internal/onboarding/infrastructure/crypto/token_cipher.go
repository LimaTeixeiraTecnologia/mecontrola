package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

const (
	aes256KeyLen = 32
	nonceLen     = 12
)

type TokenCipher struct {
	gcm cipher.AEAD
}

func NewTokenCipher(key string) (*TokenCipher, error) {
	keyBytes, err := decodeKey(key)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("onboarding/crypto: criar cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("onboarding/crypto: criar gcm: %w", err)
	}
	return &TokenCipher{gcm: gcm}, nil
}

func (c *TokenCipher) Encrypt(_ context.Context, clearToken string) (string, error) {
	if clearToken == "" {
		return "", fmt.Errorf("onboarding/crypto: token claro obrigatorio")
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("onboarding/crypto: gerar nonce: %w", err)
	}
	sealed := c.gcm.Seal(nonce, nonce, []byte(clearToken), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *TokenCipher) Decrypt(_ context.Context, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", fmt.Errorf("onboarding/crypto: token cifrado obrigatorio")
	}
	raw, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("onboarding/crypto: decodificar token cifrado: %w", err)
	}
	if len(raw) <= nonceLen {
		return "", fmt.Errorf("onboarding/crypto: token cifrado invalido")
	}
	nonce := raw[:nonceLen]
	body := raw[nonceLen:]
	clear, err := c.gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("onboarding/crypto: decifrar token: %w", err)
	}
	return string(clear), nil
}

func decodeKey(key string) ([]byte, error) {
	if key == "" {
		return nil, fmt.Errorf("onboarding/crypto: chave de token obrigatoria")
	}
	if len(key) == aes256KeyLen {
		return []byte(key), nil
	}
	decoded, err := base64.RawStdEncoding.DecodeString(key)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(key)
	}
	if err != nil {
		return nil, fmt.Errorf("onboarding/crypto: chave deve ter 32 bytes ou base64 de 32 bytes: %w", err)
	}
	if len(decoded) != aes256KeyLen {
		return nil, fmt.Errorf("onboarding/crypto: chave deve ter 32 bytes")
	}
	return decoded, nil
}
