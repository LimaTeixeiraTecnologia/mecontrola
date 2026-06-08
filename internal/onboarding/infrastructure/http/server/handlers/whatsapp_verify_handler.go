package handlers

import (
	"crypto/subtle"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type WhatsAppVerifyHandler struct {
	verifyToken string
	o11y        observability.Observability
}

func NewWhatsAppVerifyHandler(verifyToken string, o11y observability.Observability) *WhatsAppVerifyHandler {
	return &WhatsAppVerifyHandler{verifyToken: verifyToken, o11y: o11y}
}

func (h *WhatsAppVerifyHandler) Handle(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode != "subscribe" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if subtle.ConstantTimeCompare([]byte(token), []byte(h.verifyToken)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(challenge))
}
