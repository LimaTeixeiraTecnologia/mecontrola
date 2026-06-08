package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	MetaSignatureStatusValid   = "valid"
	MetaSignatureStatusInvalid = "invalid"
	MetaSignatureStatusRotated = "rotated"
)

type ctxKeyMetaSignatureStatus struct{}

func MetaSignature(secretCurrent, secretNext string) func(http.Handler) http.Handler {
	return MetaSignatureWithMetrics(secretCurrent, secretNext, nil)
}

func MetaSignatureWithMetrics(secretCurrent, secretNext string, onInvalid func()) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := RawBodyFromContext(r)
			if !ok {
				http.Error(w, `{"message":"raw body unavailable"}`, http.StatusInternalServerError)
				return
			}

			header := r.Header.Get("X-Hub-Signature-256")
			status := computeMetaSignatureStatus(raw, header, secretCurrent, secretNext)
			if status == MetaSignatureStatusInvalid {
				if onInvalid != nil {
					onInvalid()
				}
				http.Error(w, `{"message":"invalid signature"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyMetaSignatureStatus{}, status)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func MetaSignatureStatusFromContext(r *http.Request) string {
	v := r.Context().Value(ctxKeyMetaSignatureStatus{})
	s, ok := v.(string)
	if !ok {
		return MetaSignatureStatusInvalid
	}
	return s
}

func computeMetaSignatureStatus(raw []byte, header, secretCurrent, secretNext string) string {
	if secretCurrent != "" && matchMetaHMAC(raw, header, secretCurrent) {
		return MetaSignatureStatusValid
	}
	if secretNext != "" && matchMetaHMAC(raw, header, secretNext) {
		return MetaSignatureStatusRotated
	}
	return MetaSignatureStatusInvalid
}

func matchMetaHMAC(raw []byte, header, secret string) bool {
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
