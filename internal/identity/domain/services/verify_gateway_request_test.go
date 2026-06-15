package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func signWith(secret []byte, userID, timestamp string) string {
	msg := canonical(userID, timestamp)
	h := hmac.New(sha256.New, secret)
	h.Write(msg)
	return hex.EncodeToString(h.Sum(nil))
}

func TestVerifyGatewayRequest(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	window := 60 * time.Second
	current := []byte("current-secret-32-bytes-pad-aaaa")
	next := []byte("next-secret-32-bytes-padding-bbb")
	userID := "aabbccdd-0000-0000-0000-000000000001"
	ts := "1700000000"

	sigCurrent := signWith(current, userID, ts)
	sigNext := signWith(next, userID, ts)
	sigUpperUID := signWith(current, userID, ts)
	sigWrongKey := signWith([]byte("completely-different-secret-xxxxx"), userID, ts)

	tests := []struct {
		name                 string
		req                  VerifyRequest
		secrets              SecretPair
		now                  time.Time
		window               time.Duration
		wantKind             GatewayAuthResultKind
		wantTimestampFailure GatewayAuthTimestampFailureKind
	}{
		{
			name:     "happy current",
			req:      VerifyRequest{UserIDRaw: userID, SignatureRaw: sigCurrent, TimestampRaw: ts},
			secrets:  SecretPair{Current: current, Next: next},
			now:      now,
			window:   window,
			wantKind: GatewayAuthValid,
		},
		{
			name:     "happy rotated",
			req:      VerifyRequest{UserIDRaw: userID, SignatureRaw: sigNext, TimestampRaw: ts},
			secrets:  SecretPair{Current: current, Next: next},
			now:      now,
			window:   window,
			wantKind: GatewayAuthRotated,
		},
		{
			name:     "missing user_id",
			req:      VerifyRequest{UserIDRaw: "", SignatureRaw: sigCurrent, TimestampRaw: ts},
			secrets:  SecretPair{Current: current},
			now:      now,
			window:   window,
			wantKind: GatewayAuthMissingHeader,
		},
		{
			name:     "missing signature",
			req:      VerifyRequest{UserIDRaw: userID, SignatureRaw: "", TimestampRaw: ts},
			secrets:  SecretPair{Current: current},
			now:      now,
			window:   window,
			wantKind: GatewayAuthMissingHeader,
		},
		{
			name:     "missing timestamp",
			req:      VerifyRequest{UserIDRaw: userID, SignatureRaw: sigCurrent, TimestampRaw: ""},
			secrets:  SecretPair{Current: current},
			now:      now,
			window:   window,
			wantKind: GatewayAuthMissingHeader,
		},
		{
			name:                 "timestamp not numeric",
			req:                  VerifyRequest{UserIDRaw: userID, SignatureRaw: sigCurrent, TimestampRaw: "abc"},
			secrets:              SecretPair{Current: current},
			now:                  now,
			window:               window,
			wantKind:             GatewayAuthStaleTimestamp,
			wantTimestampFailure: GatewayAuthTimestampFailureInvalid,
		},
		{
			name:                 "timestamp float string",
			req:                  VerifyRequest{UserIDRaw: userID, SignatureRaw: sigCurrent, TimestampRaw: "1700000000.5"},
			secrets:              SecretPair{Current: current},
			now:                  now,
			window:               window,
			wantKind:             GatewayAuthStaleTimestamp,
			wantTimestampFailure: GatewayAuthTimestampFailureInvalid,
		},
		{
			name:                 "timestamp +61s future",
			req:                  VerifyRequest{UserIDRaw: userID, SignatureRaw: sigCurrent, TimestampRaw: "1700000062"},
			secrets:              SecretPair{Current: current},
			now:                  now,
			window:               window,
			wantKind:             GatewayAuthStaleTimestamp,
			wantTimestampFailure: GatewayAuthTimestampFailureStale,
		},
		{
			name:                 "timestamp -61s past",
			req:                  VerifyRequest{UserIDRaw: userID, SignatureRaw: sigCurrent, TimestampRaw: "1699999938"},
			secrets:              SecretPair{Current: current},
			now:                  now,
			window:               window,
			wantKind:             GatewayAuthStaleTimestamp,
			wantTimestampFailure: GatewayAuthTimestampFailureStale,
		},
		{
			name:     "signature wrong length 30 chars",
			req:      VerifyRequest{UserIDRaw: userID, SignatureRaw: "abcdef1234567890abcdef12345678", TimestampRaw: ts},
			secrets:  SecretPair{Current: current},
			now:      now,
			window:   window,
			wantKind: GatewayAuthInvalidSignature,
		},
		{
			name:     "signature invalid charset",
			req:      VerifyRequest{UserIDRaw: userID, SignatureRaw: "ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ", TimestampRaw: ts},
			secrets:  SecretPair{Current: current},
			now:      now,
			window:   window,
			wantKind: GatewayAuthInvalidSignature,
		},
		{
			name:     "signature valid hex but wrong key",
			req:      VerifyRequest{UserIDRaw: userID, SignatureRaw: sigWrongKey, TimestampRaw: ts},
			secrets:  SecretPair{Current: current},
			now:      now,
			window:   window,
			wantKind: GatewayAuthInvalidSignature,
		},
		{
			name:     "canonical uppercase UUID equals lowercase",
			req:      VerifyRequest{UserIDRaw: "AABBCCDD-0000-0000-0000-000000000001", SignatureRaw: sigUpperUID, TimestampRaw: ts},
			secrets:  SecretPair{Current: current, Next: next},
			now:      now,
			window:   window,
			wantKind: GatewayAuthValid,
		},
		{
			name: "fixed vector cross-lang",
			req: VerifyRequest{
				UserIDRaw:    "00000000-0000-0000-0000-000000000000",
				SignatureRaw: "174e5aa87139ef38ab5968b10cd88fb33d0aa084a57f30f61e3d273ad709babe",
				TimestampRaw: "1700000000",
			},
			secrets:  SecretPair{Current: []byte("test-secret-32-bytes-padding-aaaa")},
			now:      now,
			window:   window,
			wantKind: GatewayAuthValid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GatewayRequestVerifier{}.VerifyGatewayRequest(tc.req, tc.secrets, tc.now, tc.window)
			if got.Kind != tc.wantKind {
				t.Errorf("VerifyGatewayRequest() Kind = %v, want %v", got.Kind, tc.wantKind)
			}
			if got.TimestampFailure != tc.wantTimestampFailure {
				t.Errorf("VerifyGatewayRequest() TimestampFailure = %v, want %v", got.TimestampFailure, tc.wantTimestampFailure)
			}
		})
	}
}

func TestGatewayAuthResult_IsAuthorized(t *testing.T) {
	tests := []struct {
		kind GatewayAuthResultKind
		want bool
	}{
		{GatewayAuthValid, true},
		{GatewayAuthRotated, true},
		{GatewayAuthInvalidSignature, false},
		{GatewayAuthStaleTimestamp, false},
		{GatewayAuthMissingHeader, false},
	}

	for _, tc := range tests {
		r := GatewayAuthResult{Kind: tc.kind}
		if got := r.IsAuthorized(); got != tc.want {
			t.Errorf("IsAuthorized() kind=%v = %v, want %v", tc.kind, got, tc.want)
		}
	}
}
