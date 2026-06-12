package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"
)

type deleteRecurringTemplateUseCase interface {
	Execute(ctx context.Context, templateID string, version int64) error
}

type deleteRecurringTemplateRequest struct {
	Version int64 `json:"version"`
}

type DeleteRecurringTemplateHandler struct {
	usecase deleteRecurringTemplateUseCase
	o11y    observability.Observability
}

func NewDeleteRecurringTemplateHandler(uc deleteRecurringTemplateUseCase, o11y observability.Observability) *DeleteRecurringTemplateHandler {
	return &DeleteRecurringTemplateHandler{usecase: uc, o11y: o11y}
}

func (h *DeleteRecurringTemplateHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.delete_recurring_template")
	defer span.End()

	templateID := chi.URLParam(r, "id")
	if templateID == "" {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "id obrigatório", map[string]string{"code": "validation_error"})
		return
	}

	var req deleteRecurringTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido", map[string]string{"code": "validation_error"})
		return
	}

	if err := h.usecase.Execute(ctx, templateID, req.Version); err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "deleted"))
	w.WriteHeader(http.StatusNoContent)
}
