package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	tghandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/handlers"
	tgsignature "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/signature"
)

type TelegramWebhookRouter struct {
	inboundHandler      *tghandlers.InboundHandler
	secretCurrent       string
	secretNext          string
	webhookPath         string
	rateLimitMiddleware func(http.Handler) http.Handler
	onRateLimitExceeded func()
}

func NewTelegramWebhookRouter(
	inboundHandler *tghandlers.InboundHandler,
	secretCurrent string,
	secretNext string,
	webhookPath string,
	rateLimitMiddleware func(http.Handler) http.Handler,
	onRateLimitExceeded func(),
) *TelegramWebhookRouter {
	return &TelegramWebhookRouter{
		inboundHandler:      inboundHandler,
		secretCurrent:       secretCurrent,
		secretNext:          secretNext,
		webhookPath:         webhookPath,
		rateLimitMiddleware: rateLimitMiddleware,
		onRateLimitExceeded: onRateLimitExceeded,
	}
}

func (rt *TelegramWebhookRouter) Register(r chi.Router) {
	r.With(
		rt.rateLimitWithMetric,
		tgsignature.SecretToken(rt.secretCurrent, rt.secretNext),
	).Post(rt.webhookPath, rt.inboundHandler.Handle)
}

func (rt *TelegramWebhookRouter) rateLimitWithMetric(next http.Handler) http.Handler {
	inner := rt.rateLimitMiddleware(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cw := &captureWriter{ResponseWriter: w, code: http.StatusOK}
		inner.ServeHTTP(cw, r)
		if cw.code == http.StatusTooManyRequests && rt.onRateLimitExceeded != nil {
			rt.onRateLimitExceeded()
		}
	})
}
