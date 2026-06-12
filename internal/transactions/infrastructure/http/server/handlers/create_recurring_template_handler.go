package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	dtoinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type createRecurringTemplateUseCase interface {
	Execute(ctx context.Context, raw dtoinput.RawCreateRecurringTemplate) (dtooutput.RecurringTemplate, error)
}

type CreateRecurringTemplateHandler struct {
	usecase createRecurringTemplateUseCase
	o11y    observability.Observability
}

func NewCreateRecurringTemplateHandler(uc createRecurringTemplateUseCase, o11y observability.Observability) *CreateRecurringTemplateHandler {
	return &CreateRecurringTemplateHandler{usecase: uc, o11y: o11y}
}

func (h *CreateRecurringTemplateHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.create_recurring_template")
	defer span.End()

	var raw dtoinput.RawCreateRecurringTemplate
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido", map[string]string{"code": "validation_error"})
		return
	}

	out, err := h.usecase.Execute(ctx, raw)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "created"))
	responses.JSON(w, http.StatusCreated, out)
}
