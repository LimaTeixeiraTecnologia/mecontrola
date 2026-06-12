package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"

	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type listMonthlyEntriesUseCase interface {
	Execute(ctx context.Context, refMonthStr, cursor string, limit int) (dtooutput.MonthlyEntriesPage, error)
}

type ListMonthlyEntriesHandler struct {
	usecase listMonthlyEntriesUseCase
	o11y    observability.Observability
}

func NewListMonthlyEntriesHandler(uc listMonthlyEntriesUseCase, o11y observability.Observability) *ListMonthlyEntriesHandler {
	return &ListMonthlyEntriesHandler{usecase: uc, o11y: o11y}
}

func (h *ListMonthlyEntriesHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.list_monthly_entries")
	defer span.End()

	refMonthStr := chi.URLParam(r, "ref_month")
	if refMonthStr == "" {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "ref_month obrigatório", map[string]string{"code": "validation_error"})
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, parseErr := strconv.Atoi(l); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}

	out, err := h.usecase.Execute(ctx, refMonthStr, cursor, limit)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "ok"))
	responses.JSON(w, http.StatusOK, out)
}
