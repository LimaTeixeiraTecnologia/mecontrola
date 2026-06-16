package server

import (
	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
)

type WebhookRouter struct {
	webhookHandler *handlers.KiwifyWebhookHandler
	secretCurrent  string
	secretNext     string
	limiter        *middleware.RateLimiter
}

func NewWebhookRouter(
	webhookHandler *handlers.KiwifyWebhookHandler,
	secretCurrent string,
	secretNext string,
	limiter *middleware.RateLimiter,
) *WebhookRouter {
	return &WebhookRouter{
		webhookHandler: webhookHandler,
		secretCurrent:  secretCurrent,
		secretNext:     secretNext,
		limiter:        limiter,
	}
}

func (rt *WebhookRouter) Register(r chi.Router) {
	if rt.webhookHandler == nil {
		return
	}
	r.Route("/api/v1/billing/webhooks", func(sub chi.Router) {
		chain := sub.With(
			middleware.RawBody,
			middleware.HMACSignature(rt.secretCurrent, rt.secretNext),
		)
		if rt.limiter != nil {
			chain = chain.With(rt.limiter.Middleware)
		}
		chain.Post("/kiwify", rt.webhookHandler.Handle)
	})
}
