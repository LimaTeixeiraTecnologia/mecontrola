package kiwify

import (
	"crypto/subtle"
	"errors"
	"strings"
)

var (
	// ErrMissingSignature é retornado quando o header de assinatura está ausente.
	ErrMissingSignature = errors.New("kiwify webhook: header de assinatura ausente")
	// ErrInvalidSignature é retornado quando a assinatura não corresponde ao token esperado.
	ErrInvalidSignature = errors.New("kiwify webhook: assinatura inválida")
)

// SignatureVerifier define o contrato de verificação de assinatura de webhook.
// Permite evolução para HMAC-SHA256 sem mudança de RF (ADR-006).
type SignatureVerifier interface {
	Verify(payload []byte, headers map[string]string) error
}

// TokenSignatureVerifier verifica assinatura por comparação constant-time de token em header.
// Usa crypto/subtle.ConstantTimeCompare para prevenção de timing attack (RF-02, ADR-006).
// O lookup do header é case-insensitive para compatibilidade com proxies que normalizam headers.
type TokenSignatureVerifier struct {
	expectedToken string
	headerName    string
}

// NewTokenSignatureVerifier cria um TokenSignatureVerifier.
// headerName é armazenado em lowercase para lookup case-insensitive.
func NewTokenSignatureVerifier(expectedToken, headerName string) *TokenSignatureVerifier {
	return &TokenSignatureVerifier{
		expectedToken: expectedToken,
		headerName:    strings.ToLower(headerName),
	}
}

// Verify verifica a assinatura do webhook por comparação constant-time.
// Retorna ErrMissingSignature se o header estiver ausente e ErrInvalidSignature se o valor não bater.
func (v *TokenSignatureVerifier) Verify(_ []byte, headers map[string]string) error {
	received := lookupHeaderCaseInsensitive(headers, v.headerName)
	if received == "" {
		return ErrMissingSignature
	}
	if subtle.ConstantTimeCompare([]byte(received), []byte(v.expectedToken)) != 1 {
		return ErrInvalidSignature
	}
	return nil
}

func lookupHeaderCaseInsensitive(headers map[string]string, name string) string {
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return ""
}
