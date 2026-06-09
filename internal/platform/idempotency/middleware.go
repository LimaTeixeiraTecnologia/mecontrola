package idempotency

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
	"unicode"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

const (
	headerIdempotencyKey = "Idempotency-Key"
	maxKeyLength         = 128
)

type errorBody struct {
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, msg string) {
	b, _ := json.Marshal(errorBody{Message: msg})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func Middleware(scope string, storage Storage, ttl time.Duration, o11y observability.Observability) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := o11y.Tracer().Start(r.Context(), "card.middleware.idempotency")
			defer span.End()

			key := r.Header.Get(headerIdempotencyKey)
			if key == "" {
				span.SetAttributes(observability.String("outcome", "error"))
				writeJSON(w, http.StatusBadRequest, "missing_idempotency_key")
				return
			}

			if len(key) > maxKeyLength || !isASCII(key) {
				span.SetAttributes(observability.String("outcome", "error"))
				writeJSON(w, http.StatusBadRequest, "invalid_idempotency_key")
				return
			}

			principal, ok := auth.FromContext(ctx)
			if !ok {
				span.SetAttributes(observability.String("outcome", "error"))
				writeJSON(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			userID := principal.UserID.String()

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				span.SetAttributes(observability.String("outcome", "error"))
				writeJSON(w, http.StatusInternalServerError, "internal_error")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			hash := sha256.Sum256(bodyBytes)
			requestHash := hex.EncodeToString(hash[:])

			existing, err := storage.Get(ctx, scope, key, userID)
			if err == nil {
				if existing.RequestHash == requestHash {
					span.SetAttributes(
						observability.String("outcome", "replay"),
						observability.String("user_id", userID),
					)
					o11y.Logger().Info(ctx, "card.idempotency.replay",
						observability.String("scope", scope),
						observability.String("key", key),
						observability.String("user_id", userID),
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(existing.ResponseStatus)
					_, _ = w.Write(existing.ResponseBody)
					return
				}
				span.SetAttributes(
					observability.String("outcome", "conflict"),
					observability.String("user_id", userID),
				)
				writeJSON(w, http.StatusConflict, "idempotency_conflict")
				return
			}

			if !errors.Is(err, ErrNotFound) {
				span.SetAttributes(observability.String("outcome", "error"))
				writeJSON(w, http.StatusInternalServerError, "internal_error")
				return
			}

			span.SetAttributes(
				observability.String("outcome", "miss"),
				observability.String("user_id", userID),
			)

			ic := IdempotencyContext{
				Scope:       scope,
				Key:         key,
				UserID:      userID,
				RequestHash: requestHash,
				ExpiresAt:   time.Now().UTC().Add(ttl),
			}
			ctx = WithContext(ctx, ic)

			rec := newResponseRecorder(w)
			next.ServeHTTP(rec, r.WithContext(ctx))

			if rec.overflow {
				o11y.Logger().Warn(ctx, "card.idempotency.body_overflow",
					observability.String("scope", scope),
					observability.String("key", key),
					observability.String("user_id", userID),
				)
				span.SetAttributes(observability.String("outcome", "overflow"))
				writeJSON(w, http.StatusInternalServerError, "internal_error")
				return
			}

			rec.flush()

			status := rec.status
			if status >= 400 && status <= 499 {
				putRec := Record{
					Scope:          scope,
					Key:            key,
					UserID:         userID,
					RequestHash:    requestHash,
					ResponseStatus: status,
					ResponseBody:   rec.buf.Bytes(),
					ExpiresAt:      ic.ExpiresAt,
				}
				if putErr := storage.Put(ctx, putRec); putErr != nil {
					o11y.Logger().Warn(ctx, "card.idempotency.put_4xx_failed",
						observability.String("scope", scope),
						observability.String("key", key),
						observability.String("user_id", userID),
						observability.Error(putErr),
					)
				}
			}
		})
	}
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}
