package server

import (
	"github.com/go-chi/chi/v5"

	wahandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
)

type WhatsAppWebhookRouter struct {
	verifyHandler  *wahandlers.VerifyHandler
	inboundHandler *wahandlers.InboundHandler
	secretCurrent  string
	secretNext     string
}

func NewWhatsAppWebhookRouter(
	verifyHandler *wahandlers.VerifyHandler,
	inboundHandler *wahandlers.InboundHandler,
	secretCurrent string,
	secretNext string,
) *WhatsAppWebhookRouter {
	return &WhatsAppWebhookRouter{
		verifyHandler:  verifyHandler,
		inboundHandler: inboundHandler,
		secretCurrent:  secretCurrent,
		secretNext:     secretNext,
	}
}

func (rt *WhatsAppWebhookRouter) Register(r chi.Router) {
	r.Route("/api/v1/whatsapp", func(sub chi.Router) {
		sub.Get("/verify", rt.verifyHandler.Handle)
		sub.With(
			signature.Compose(rt.secretCurrent, rt.secretNext, nil),
		).Post("/inbound", rt.inboundHandler.Handle)
	})
}
