package signature

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	StatusValid   = "valid"
	StatusInvalid = "invalid"
	StatusRotated = "rotated"
)

type ctxKeyHMACStatus struct{}

func HMAC(secretCurrent, secretNext string) func(http.Handler) http.Handler {
	return HMACWithMetrics(secretCurrent, secretNext, nil, nil)
}

func HMACWithMetrics(secretCurrent, secretNext string, onInvalid func(), onStatus func(status string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := RawBodyFromContext(r)
			if !ok {
				http.Error(w, `{"message":"raw body unavailable"}`, http.StatusInternalServerError)
				return
			}

			header := r.Header.Get("X-Hub-Signature-256")
			status := computeHMACStatus(raw, header, secretCurrent, secretNext)
			if onStatus != nil {
				onStatus(status)
			}
			if status == StatusInvalid {
				if onInvalid != nil {
					onInvalid()
				}
				http.Error(w, `{"message":"invalid signature"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyHMACStatus{}, status)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func StatusFromContext(r *http.Request) string {
	v := r.Context().Value(ctxKeyHMACStatus{})
	s, ok := v.(string)
	if !ok {
		return StatusInvalid
	}
	return s
}

func computeHMACStatus(raw []byte, header, secretCurrent, secretNext string) string {
	if secretCurrent != "" && matchHMAC(raw, header, secretCurrent) {
		return StatusValid
	}
	if secretNext != "" && matchHMAC(raw, header, secretNext) {
		return StatusRotated
	}
	return StatusInvalid
}

func matchHMAC(raw []byte, header, secret string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	hexSig := header[len(prefix):]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(hexSig))
}
