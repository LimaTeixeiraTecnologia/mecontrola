package server

import (
	"net/http"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
)

// KiwifyRouteRegistrar registra a rota POST /webhooks/kiwify no roteador Chi (ADR-002).
// O handler é definido via SetHandler após a construção para suportar wiring lazy no bootstrap.
type KiwifyRouteRegistrar struct {
	handler atomic.Pointer[KiwifyWebhookHandler]
}

// NewKiwifyRouteRegistrar cria um KiwifyRouteRegistrar.
// handler pode ser nil quando o registrar for criado antes do use case (wiring lazy).
func NewKiwifyRouteRegistrar(handler *KiwifyWebhookHandler) *KiwifyRouteRegistrar {
	r := &KiwifyRouteRegistrar{}
	if handler != nil {
		r.handler.Store(handler)
	}
	return r
}

// Register associa a rota POST /webhooks/kiwify ao handler (ADR-002).
func (r *KiwifyRouteRegistrar) Register(router chi.Router) {
	router.Post("/webhooks/kiwify", func(w http.ResponseWriter, req *http.Request) {
		h := r.handler.Load()
		if h == nil {
			writeWebhookJSON(w, http.StatusServiceUnavailable)
			return
		}
		h.ServeHTTP(w, req)
	})
}

// SetHandler define o handler usado pelo registrar. Seguro para uso concorrente.
func (r *KiwifyRouteRegistrar) SetHandler(handler *KiwifyWebhookHandler) {
	r.handler.Store(handler)
}
