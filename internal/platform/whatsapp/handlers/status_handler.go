package handlers

import (
	"context"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/status"
)

type statusRecorder interface {
	Execute(ctx context.Context, statuses []status.MessageStatus) error
}

type StatusHandler struct {
	recorder statusRecorder
	o11y     observability.Observability
}

func NewStatusHandler(recorder statusRecorder, o11y observability.Observability) *StatusHandler {
	return &StatusHandler{recorder: recorder, o11y: o11y}
}

func (h *StatusHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "whatsapp.handler.status")
	defer span.End()

	raw, ok := signature.RawBodyFromContext(r)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	statuses, err := status.ExtractStatuses(raw)
	if err != nil {
		h.o11y.Logger().Warn(ctx, "whatsapp.handler.status.parse_failed",
			observability.Error(err),
		)
		w.WriteHeader(http.StatusOK)
		return
	}

	if len(statuses) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.recorder.Execute(ctx, statuses); err != nil {
		h.o11y.Logger().Error(ctx, "whatsapp.handler.status.record_failed",
			observability.Error(err),
		)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}
