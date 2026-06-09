package middleware

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

const headerUserID = "X-User-ID"

type injectPrincipalMiddleware struct {
	o11y observability.Observability
}

func InjectPrincipalFromHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get(headerUserID)
		if raw == "" {
			next.ServeHTTP(w, r)
			return
		}
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			next.ServeHTTP(w, r)
			return
		}
		p := auth.Principal{UserID: id, Source: auth.SourceHeader}
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
	})
}

func InjectPrincipalFromHeaderWithO11y(o11y observability.Observability) func(http.Handler) http.Handler {
	m := &injectPrincipalMiddleware{o11y: o11y}
	return m.handler
}

func (m *injectPrincipalMiddleware) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := m.o11y.Tracer().Start(r.Context(), "auth.inject_principal_from_header")
		defer span.End()

		raw := r.Header.Get(headerUserID)
		if raw == "" {
			span.SetAttributes(observability.String("result", "missing"))
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			span.SetAttributes(observability.String("result", "invalid"))
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		p := auth.Principal{UserID: id, Source: auth.SourceHeader}
		span.SetAttributes(
			observability.String("principal.source", string(auth.SourceHeader)),
			observability.String("result", "injected"),
		)
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(ctx, p)))
	})
}
