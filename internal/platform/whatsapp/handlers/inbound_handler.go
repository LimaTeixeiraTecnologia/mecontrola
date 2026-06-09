package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
)

type whatsAppDispatcher interface {
	Route(ctx context.Context, raw json.RawMessage) (dispatcher.RouteOutcome, error)
}

type InboundHandler struct {
	dispatcher whatsAppDispatcher
	o11y       observability.Observability
}

func NewInboundHandler(d whatsAppDispatcher, o11y observability.Observability) *InboundHandler {
	return &InboundHandler{dispatcher: d, o11y: o11y}
}

func (h *InboundHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "whatsapp.handler.inbound")
	defer span.End()

	raw, ok := signature.RawBodyFromContext(r)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err := h.dispatcher.Route(ctx, json.RawMessage(raw))
	if err != nil {
		h.o11y.Logger().Error(ctx, "whatsapp.handler.inbound.route_failed",
			observability.Error(err),
		)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
