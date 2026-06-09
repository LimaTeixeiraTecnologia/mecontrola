package handlers

import (
	"crypto/subtle"
	"net/http"
)

type VerifyHandler struct {
	verifyToken string
}

func NewVerifyHandler(verifyToken string) *VerifyHandler {
	return &VerifyHandler{verifyToken: verifyToken}
}

func (h *VerifyHandler) Handle(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode != "subscribe" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if subtle.ConstantTimeCompare([]byte(token), []byte(h.verifyToken)) != 1 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(challenge))
}
