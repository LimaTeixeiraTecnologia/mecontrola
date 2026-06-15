package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

const canonicalSeparator = "."

type VerifyRequest struct {
	UserIDRaw    string
	SignatureRaw string
	TimestampRaw string
}

type SecretPair struct {
	Current []byte
	Next    []byte
}

func VerifyGatewayRequest(req VerifyRequest, secrets SecretPair, now time.Time, window time.Duration) GatewayAuthResult {
	if req.UserIDRaw == "" || req.SignatureRaw == "" || req.TimestampRaw == "" {
		return GatewayAuthResult{Kind: GatewayAuthMissingHeader}
	}

	_, err := valueobjects.NewGatewayTimestamp(req.TimestampRaw, now, window)
	if err != nil {
		if errors.Is(err, valueobjects.ErrGatewayTimestampStale) {
			return GatewayAuthResult{Kind: GatewayAuthStaleTimestamp, TimestampFailure: GatewayAuthTimestampFailureStale}
		}
		return GatewayAuthResult{Kind: GatewayAuthStaleTimestamp, TimestampFailure: GatewayAuthTimestampFailureInvalid}
	}

	sig, err := valueobjects.NewGatewaySignature(req.SignatureRaw)
	if err != nil {
		return GatewayAuthResult{Kind: GatewayAuthInvalidSignature}
	}

	msg := canonical(req.UserIDRaw, req.TimestampRaw)

	if hmac.Equal(sig.Bytes(), computeHMAC(secrets.Current, msg)) {
		return GatewayAuthResult{Kind: GatewayAuthValid}
	}

	if len(secrets.Next) > 0 && hmac.Equal(sig.Bytes(), computeHMAC(secrets.Next, msg)) {
		return GatewayAuthResult{Kind: GatewayAuthRotated}
	}

	return GatewayAuthResult{Kind: GatewayAuthInvalidSignature}
}

func canonical(userIDRaw, timestampRaw string) []byte {
	lower := strings.ToLower(userIDRaw)
	buf := make([]byte, len(lower)+len(canonicalSeparator)+len(timestampRaw))
	n := copy(buf, lower)
	n += copy(buf[n:], canonicalSeparator)
	copy(buf[n:], timestampRaw)
	return buf
}

func computeHMAC(secret, msg []byte) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write(msg)
	return h.Sum(nil)
}
