package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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

			header := r.Header.Get("X-Kiwify-Signature")
			if header == "" {
				header = r.URL.Query().Get("signature")
			}

			status := computeSignatureStatus(raw, header, secretCurrent, secretNext)
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

func computeSignatureStatus(raw []byte, header, secretCurrent, secretNext string) string {
	if secretCurrent != "" && matchHMAC(raw, header, secretCurrent) {
		return SignatureStatusValid
	}
	if secretNext != "" && matchHMAC(raw, header, secretNext) {
		return SignatureStatusRotated
	}
	return SignatureStatusInvalid
}

func matchHMAC(raw []byte, header, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(header))
}
