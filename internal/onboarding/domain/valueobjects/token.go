package valueobjects

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrTokenEmpty   = errors.New("onboarding: token is empty")
	ErrTokenInvalid = errors.New("onboarding: token format invalid")
)

const tokenByteLen = 32

type Token struct {
	clear []byte
	hash  [sha256.Size]byte
}

func NewToken() (Token, error) {
	b := make([]byte, tokenByteLen)
	if _, err := rand.Read(b); err != nil {
		return Token{}, fmt.Errorf("onboarding: generate token: %w", err)
	}
	h := sha256.Sum256(b)
	return Token{clear: b, hash: h}, nil
}

func TokenFromClear(clear string) (Token, error) {
	if clear == "" {
		return Token{}, ErrTokenEmpty
	}
	b, err := base64.RawURLEncoding.DecodeString(clear)
	if err != nil || len(b) != tokenByteLen {
		return Token{}, fmt.Errorf("onboarding: %q: %w", clear, ErrTokenInvalid)
	}
	h := sha256.Sum256(b)
	return Token{clear: b, hash: h}, nil
}

func (t Token) ClearText() string {
	return base64.RawURLEncoding.EncodeToString(t.clear)
}

func (t Token) Hash() []byte {
	h := t.hash
	return h[:]
}

func (t Token) HashHex() string {
	return hex.EncodeToString(t.hash[:])
}

func (t Token) HashPrefix() string {
	return TokenHashPrefix(t.hash[:])
}

func (t Token) String() string {
	return "[REDACTED]"
}
