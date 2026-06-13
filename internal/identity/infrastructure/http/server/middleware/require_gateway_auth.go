package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
)

const (
	headerGatewayAuth  = "X-Gateway-Auth"
	headerGatewayTS    = "X-Gateway-Timestamp"
	headerForwardedFor = "X-Forwarded-For"
	headerRequestID    = "X-Request-Id"
)

var (
	gatewayAuthBuckets    = []float64{0.0001, 0.0005, 0.001, 0.002, 0.005, 0.01, 0.05}
	errorBodyUnauthorized = []byte(`{"error":"unauthorized"}`)
)

type gatewayAuthFailureLogger interface {
	Handle(ctx context.Context, in input.RecordGatewayAuthFailureInput) error
}

type RequireGatewayAuthDeps struct {
	Secrets       services.SecretPair
	Window        time.Duration
	FailureLogger gatewayAuthFailureLogger
	O11y          observability.Observability
}

func RequireGatewayAuth(deps RequireGatewayAuthDeps) func(http.Handler) http.Handler {
	total := deps.O11y.Metrics().Counter("identity_gateway_auth_total", "Total gateway auth requests by result", "1")
	dur := deps.O11y.Metrics().HistogramWithBuckets("identity_gateway_auth_duration_seconds", "Gateway auth request duration", "s", gatewayAuthBuckets)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx, span := deps.O11y.Tracer().Start(r.Context(), "auth.require_gateway_auth")
			defer span.End()
			uid := r.Header.Get(headerUserID)
			result := services.VerifyGatewayRequest(
				services.VerifyRequest{UserIDRaw: uid, SignatureRaw: r.Header.Get(headerGatewayAuth), TimestampRaw: r.Header.Get(headerGatewayTS)},
				deps.Secrets, time.Now().UTC(), deps.Window,
			)
			elapsed := time.Since(start).Seconds()
			switch result.Kind {
			case services.GatewayAuthValid:
				total.Increment(ctx, observability.String("result", "valid"))
				dur.Record(ctx, elapsed, observability.String("result", "valid"))
				span.SetAttributes(observability.String("result", "valid"), observability.Bool("rotated", false), observability.Bool("has_user_id", uid != ""))
				next.ServeHTTP(w, r.WithContext(ctx))
			case services.GatewayAuthRotated:
				total.Increment(ctx, observability.String("result", "rotated"))
				dur.Record(ctx, elapsed, observability.String("result", "rotated"))
				span.SetAttributes(observability.String("result", "rotated"), observability.Bool("rotated", true), observability.Bool("has_user_id", uid != ""))
				next.ServeHTTP(w, r.WithContext(ctx))
			case services.GatewayAuthMissingHeader:
				recordFailure(ctx, r, w, "missing_header", "gateway_missing_header", uid, elapsed, deps, total, dur, span)
			case services.GatewayAuthStaleTimestamp:
				recordFailure(ctx, r, w, "stale_timestamp", "gateway_stale_timestamp", uid, elapsed, deps, total, dur, span)
			case services.GatewayAuthInvalidSignature:
				recordFailure(ctx, r, w, "invalid_signature", "gateway_invalid_signature", uid, elapsed, deps, total, dur, span)
			case services.GatewayAuthInvalidTimestamp:
				recordFailure(ctx, r, w, "invalid_timestamp", "gateway_invalid_timestamp", uid, elapsed, deps, total, dur, span)
			}
		})
	}
}

func recordFailure(ctx context.Context, r *http.Request, w http.ResponseWriter, lbl, reason, uid string, elapsed float64, deps RequireGatewayAuthDeps, total observability.Counter, dur observability.Histogram, span observability.Span) {
	total.Increment(ctx, observability.String("result", lbl))
	dur.Record(ctx, elapsed, observability.String("result", lbl))
	span.SetAttributes(observability.String("result", lbl), observability.Bool("rotated", false), observability.Bool("has_user_id", uid != ""))
	cip := lastForwardedFor(r.Header.Get(headerForwardedFor))
	rid := r.Header.Get(headerRequestID)
	deps.O11y.Logger().Warn(ctx, "gateway auth failed", observability.String("result", lbl), observability.String("request_id", rid), observability.String("client_ip", cip), observability.String("user_id_prefix", userIDPrefix(uid)))
	if err := deps.FailureLogger.Handle(ctx, input.RecordGatewayAuthFailureInput{UserIDRaw: uid, Reason: reason, RequestID: rid, ClientIPRaw: cip}); err != nil {
		deps.O11y.Logger().Warn(ctx, "gateway auth: failure logger publish error", observability.String("result", lbl))
	}
	respondUnauthorized(w)
}

func respondUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write(errorBodyUnauthorized)
}

func userIDPrefix(raw string) string {
	if len(raw) < 8 {
		return ""
	}
	return raw[:8]
}

func lastForwardedFor(xff string) string {
	if strings.TrimSpace(xff) == "" {
		return ""
	}
	parts := strings.Split(xff, ",")
	return strings.TrimSpace(parts[len(parts)-1])
}
