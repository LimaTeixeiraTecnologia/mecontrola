package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	wahandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
)

type captureWriter struct {
	http.ResponseWriter
	code int
}

func (cw *captureWriter) WriteHeader(code int) {
	cw.code = code
	cw.ResponseWriter.WriteHeader(code)
}

type WhatsAppWebhookRouter struct {
	verifyHandler       *wahandlers.VerifyHandler
	inboundHandler      *wahandlers.InboundHandler
	secretCurrent       string
	secretNext          string
	rateLimitMiddleware func(http.Handler) http.Handler
	onRateLimitExceeded func()
}

func NewWhatsAppWebhookRouter(
	verifyHandler *wahandlers.VerifyHandler,
	inboundHandler *wahandlers.InboundHandler,
	secretCurrent string,
	secretNext string,
	rateLimitMiddleware func(http.Handler) http.Handler,
	onRateLimitExceeded func(),
) *WhatsAppWebhookRouter {
	return &WhatsAppWebhookRouter{
		verifyHandler:       verifyHandler,
		inboundHandler:      inboundHandler,
		secretCurrent:       secretCurrent,
		secretNext:          secretNext,
		rateLimitMiddleware: rateLimitMiddleware,
		onRateLimitExceeded: onRateLimitExceeded,
	}
}

func (rt *WhatsAppWebhookRouter) Register(r chi.Router) {
	r.Route("/api/v1/whatsapp", func(sub chi.Router) {
		sub.Get("/verify", rt.verifyHandler.Handle)
		sub.With(
			rt.rateLimitWithMetric,
			signature.Compose(rt.secretCurrent, rt.secretNext, nil),
		).Post("/inbound", rt.inboundHandler.Handle)
	})
}

func (rt *WhatsAppWebhookRouter) rateLimitWithMetric(next http.Handler) http.Handler {
	inner := rt.rateLimitMiddleware(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cw := &captureWriter{ResponseWriter: w, code: http.StatusOK}
		inner.ServeHTTP(cw, r)
		if cw.code == http.StatusTooManyRequests && rt.onRateLimitExceeded != nil {
			rt.onRateLimitExceeded()
		}
	})
}
