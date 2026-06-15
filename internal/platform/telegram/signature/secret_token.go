package signature

import (
	"bytes"
	"context"
	"crypto/subtle"
	"io"
	"net/http"
)

const (
	StatusValid   = "valid"
	StatusInvalid = "invalid"
	StatusRotated = "rotated"

	HeaderSecretToken = "X-Telegram-Bot-Api-Secret-Token"

	maxTelegramBodyBytes = 1 << 20
)

type ctxKeyRawBody struct{}

type ctxKeyStatus struct{}

func SecretToken(secretCurrent, secretNext string) func(http.Handler) http.Handler {
	return SecretTokenWithMetrics(secretCurrent, secretNext, nil, nil)
}

func SecretTokenWithMetrics(secretCurrent, secretNext string, onInvalid func(), onStatus func(status string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limited := io.LimitReader(r.Body, maxTelegramBodyBytes+1)
			raw, err := io.ReadAll(limited)
			if err != nil {
				http.Error(w, `{"message":"failed to read body"}`, http.StatusInternalServerError)
				return
			}
			if len(raw) > maxTelegramBodyBytes {
				http.Error(w, `{"message":"payload too large"}`, http.StatusRequestEntityTooLarge)
				return
			}

			header := r.Header.Get(HeaderSecretToken)
			status := computeStatus(header, secretCurrent, secretNext)
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

			r.Body = io.NopCloser(bytes.NewReader(raw))
			ctx := context.WithValue(r.Context(), ctxKeyRawBody{}, raw)
			ctx = context.WithValue(ctx, ctxKeyStatus{}, status)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RawBodyFromContext(r *http.Request) ([]byte, bool) {
	v := r.Context().Value(ctxKeyRawBody{})
	b, ok := v.([]byte)
	return b, ok
}

func StatusFromContext(r *http.Request) string {
	v := r.Context().Value(ctxKeyStatus{})
	s, ok := v.(string)
	if !ok {
		return StatusInvalid
	}
	return s
}

func computeStatus(header, secretCurrent, secretNext string) string {
	if header == "" {
		return StatusInvalid
	}
	if secretCurrent != "" && constantTimeEqual(header, secretCurrent) {
		return StatusValid
	}
	if secretNext != "" && constantTimeEqual(header, secretNext) {
		return StatusRotated
	}
	return StatusInvalid
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
