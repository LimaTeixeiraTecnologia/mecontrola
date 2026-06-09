package middleware

import (
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

const unauthorizedBody = `{"message":"unauthorized"}`

type requireUserMiddleware struct {
	o11y observability.Observability
}

func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.FromContext(r.Context()); !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(unauthorizedBody))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireUserWithO11y(o11y observability.Observability) func(http.Handler) http.Handler {
	m := &requireUserMiddleware{o11y: o11y}
	return m.handler
}

func (m *requireUserMiddleware) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := m.o11y.Tracer().Start(r.Context(), "auth.require_user")
		defer span.End()

		if _, ok := auth.FromContext(ctx); !ok {
			span.SetAttributes(observability.String("result", "unauthorized"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(unauthorizedBody))
			return
		}

		span.SetAttributes(observability.String("result", "pass"))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
