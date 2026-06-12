package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"

	dtoinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type updateRecurringTemplateUseCase interface {
	Execute(ctx context.Context, templateID string, raw dtoinput.RawUpdateRecurringTemplate) (dtooutput.RecurringTemplate, error)
}

type UpdateRecurringTemplateHandler struct {
	usecase updateRecurringTemplateUseCase
	o11y    observability.Observability
}

func NewUpdateRecurringTemplateHandler(uc updateRecurringTemplateUseCase, o11y observability.Observability) *UpdateRecurringTemplateHandler {
	return &UpdateRecurringTemplateHandler{usecase: uc, o11y: o11y}
}

func (h *UpdateRecurringTemplateHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.update_recurring_template")
	defer span.End()

	templateID := chi.URLParam(r, "id")
	if templateID == "" {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "id obrigatório", map[string]string{"code": "validation_error"})
		return
	}

	var raw dtoinput.RawUpdateRecurringTemplate
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido", map[string]string{"code": "validation_error"})
		return
	}

	out, err := h.usecase.Execute(ctx, templateID, raw)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "updated"))
	responses.JSON(w, http.StatusOK, out)
}
