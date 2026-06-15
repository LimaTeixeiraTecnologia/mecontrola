package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/signature"
)

type telegramDispatcher interface {
	Route(ctx context.Context, raw json.RawMessage) (dispatcher.RouteOutcome, error)
}

type InboundHandler struct {
	dispatcher telegramDispatcher
	o11y       observability.Observability
}

func NewInboundHandler(d telegramDispatcher, o11y observability.Observability) *InboundHandler {
	return &InboundHandler{dispatcher: d, o11y: o11y}
}

func (h *InboundHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "telegram.handler.inbound")
	defer span.End()

	raw, ok := signature.RawBodyFromContext(r)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := h.dispatcher.Route(ctx, json.RawMessage(raw)); err != nil {
		h.o11y.Logger().Error(ctx, "telegram.handler.inbound.route_failed",
			observability.Error(err),
		)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
