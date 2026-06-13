package valueobjects

import (
	"encoding/hex"
	"errors"
	"fmt"
)

var ErrGatewaySignatureInvalid = errors.New("identity: gateway signature invalid")

type GatewaySignature struct {
	raw []byte
}

func NewGatewaySignature(input string) (GatewaySignature, error) {
	if len(input) != 64 {
		return GatewaySignature{}, fmt.Errorf("identity: %w: expected 64 hex chars, got %d", ErrGatewaySignatureInvalid, len(input))
	}
	for i := range len(input) {
		c := input[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return GatewaySignature{}, fmt.Errorf("identity: %w: invalid char %q at pos %d", ErrGatewaySignatureInvalid, c, i)
		}
	}
	b, _ := hex.DecodeString(input)
	return GatewaySignature{raw: b}, nil
}

func (s GatewaySignature) Bytes() []byte {
	out := make([]byte, len(s.raw))
	copy(out, s.raw)
	return out
}

func (s GatewaySignature) IsZero() bool {
	return len(s.raw) == 0
}
