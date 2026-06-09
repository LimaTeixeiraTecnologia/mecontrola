package signature

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

const maxMetaBodyBytes = 256 * 1024

type ctxKeyRawBody struct{}

func RawBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limited := io.LimitReader(r.Body, maxMetaBodyBytes+1)
		raw, err := io.ReadAll(limited)
		if err != nil {
			http.Error(w, `{"message":"failed to read body"}`, http.StatusInternalServerError)
			return
		}
		if len(raw) > maxMetaBodyBytes {
			http.Error(w, `{"message":"payload too large"}`, http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		ctx := context.WithValue(r.Context(), ctxKeyRawBody{}, raw)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RawBodyFromContext(r *http.Request) ([]byte, bool) {
	v := r.Context().Value(ctxKeyRawBody{})
	b, ok := v.([]byte)
	return b, ok
}
