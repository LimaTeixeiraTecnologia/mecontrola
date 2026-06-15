package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
)

const (
	SignatureStatusValid   = "valid"
	SignatureStatusInvalid = "invalid"
	SignatureStatusRotated = "rotated"
)

type ctxKeySignatureStatus struct{}

func HMACSignature(secretCurrent, secretNext string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := RawBodyFromContext(r)
			if !ok {
				http.Error(w, `{"message":"raw body unavailable"}`, http.StatusInternalServerError)
				return
			}

			received := r.URL.Query().Get("signature")
			if received == "" {
				received = r.Header.Get("X-Kiwify-Signature")
			}

			status := computeSignatureStatus(raw, received, secretCurrent, secretNext)
			if status == SignatureStatusInvalid {
				http.Error(w, `{"message":"invalid signature"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeySignatureStatus{}, status)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func SignatureStatusFromContext(r *http.Request) string {
	v := r.Context().Value(ctxKeySignatureStatus{})
	s, ok := v.(string)
	if !ok {
		return SignatureStatusInvalid
	}
	return s
}

func computeSignatureStatus(raw []byte, received, secretCurrent, secretNext string) string {
	if secretCurrent != "" && matchHMAC(raw, received, secretCurrent) {
		return SignatureStatusValid
	}
	if secretNext != "" && matchHMAC(raw, received, secretNext) {
		return SignatureStatusRotated
	}
	return SignatureStatusInvalid
}

func matchHMAC(raw []byte, received, secret string) bool {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(raw)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(received))
}
