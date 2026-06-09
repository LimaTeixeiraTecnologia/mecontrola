package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/middleware"
)

type PublicRouter struct {
	checkoutHandler *handlers.CreateCheckoutHandler
	stateHandler    *handlers.TokenStateHandler
	checkoutLimiter *middleware.RateLimiter
	stateLimiter    *middleware.RateLimiter
	corsOrigins     []string
}

func NewPublicRouter(
	checkoutHandler *handlers.CreateCheckoutHandler,
	stateHandler *handlers.TokenStateHandler,
	checkoutLimiter *middleware.RateLimiter,
	stateLimiter *middleware.RateLimiter,
	corsOrigins []string,
) *PublicRouter {
	return &PublicRouter{
		checkoutHandler: checkoutHandler,
		stateHandler:    stateHandler,
		checkoutLimiter: checkoutLimiter,
		stateLimiter:    stateLimiter,
		corsOrigins:     corsOrigins,
	}
}

func (rt *PublicRouter) Register(r chi.Router) {
	r.Route("/api/v1/onboarding", func(sub chi.Router) {
		sub.With(
			rt.checkoutLimiter.Middleware,
			rt.corsMiddleware,
			chiMiddleware.AllowContentType("application/json"),
		).Post("/checkout", rt.checkoutHandler.Handle)

		sub.With(
			rt.stateLimiter.Middleware,
		).Get("/tokens/{token}/state", rt.stateHandler.Handle)
	})
}

func (rt *PublicRouter) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && rt.isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rt *PublicRouter) isAllowedOrigin(origin string) bool {
	for _, allowed := range rt.corsOrigins {
		if allowed == origin {
			return true
		}
	}
	return false
}
