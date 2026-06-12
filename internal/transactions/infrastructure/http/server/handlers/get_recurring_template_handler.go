package handlers

import (
	"context"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"

	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type getRecurringTemplateUseCase interface {
	Execute(ctx context.Context, templateID string) (dtooutput.RecurringTemplate, error)
}

type GetRecurringTemplateHandler struct {
	usecase getRecurringTemplateUseCase
	o11y    observability.Observability
}

func NewGetRecurringTemplateHandler(uc getRecurringTemplateUseCase, o11y observability.Observability) *GetRecurringTemplateHandler {
	return &GetRecurringTemplateHandler{usecase: uc, o11y: o11y}
}

func (h *GetRecurringTemplateHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.get_recurring_template")
	defer span.End()

	templateID := chi.URLParam(r, "id")
	if templateID == "" {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "id obrigatório", map[string]string{"code": "validation_error"})
		return
	}

	out, err := h.usecase.Execute(ctx, templateID)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "ok"))
	responses.JSON(w, http.StatusOK, out)
}
