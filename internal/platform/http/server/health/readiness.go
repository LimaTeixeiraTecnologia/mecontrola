package health

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type ReadinessRouter struct {
	ctx context.Context
}

func NewReadinessRouter(ctx context.Context) *ReadinessRouter {
	return &ReadinessRouter{ctx: ctx}
}

func (rt *ReadinessRouter) Register(r chi.Router) {
	r.Get("/readiness", func(w http.ResponseWriter, _ *http.Request) {
		select {
		case <-rt.ctx.Done():
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		select {
		case <-rt.ctx.Done():
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
	r.Get("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
