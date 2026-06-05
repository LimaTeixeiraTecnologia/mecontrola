package server

import (
	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
)

type UserRouter struct {
	upsertHandler *handlers.UpsertUserByWhatsAppHandler
}

func NewUserRouter(upsert *handlers.UpsertUserByWhatsAppHandler) *UserRouter {
	return &UserRouter{upsertHandler: upsert}
}

func (rt *UserRouter) Register(r chi.Router) {
	if rt.upsertHandler == nil {
		return
	}
	r.Route("/api/v1/identity/users", func(sub chi.Router) {
		sub.Post("/", rt.upsertHandler.Handle)
	})
}
