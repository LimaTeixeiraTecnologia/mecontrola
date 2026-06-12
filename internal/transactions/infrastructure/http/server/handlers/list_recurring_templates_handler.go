package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type listRecurringTemplatesUseCase interface {
	Execute(ctx context.Context, activeOnly bool, cursor string, limit int) (usecases.RecurringTemplatePage, error)
}

type ListRecurringTemplatesHandler struct {
	usecase listRecurringTemplatesUseCase
	o11y    observability.Observability
}

func NewListRecurringTemplatesHandler(uc listRecurringTemplatesUseCase, o11y observability.Observability) *ListRecurringTemplatesHandler {
	return &ListRecurringTemplatesHandler{usecase: uc, o11y: o11y}
}

func (h *ListRecurringTemplatesHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.list_recurring_templates")
	defer span.End()

	activeOnly := r.URL.Query().Get("active") == "true"
	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, parseErr := strconv.Atoi(l); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}

	out, err := h.usecase.Execute(ctx, activeOnly, cursor, limit)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "ok"))
	responses.JSON(w, http.StatusOK, out)
}
