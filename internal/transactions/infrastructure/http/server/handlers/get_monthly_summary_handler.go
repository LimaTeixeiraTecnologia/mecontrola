package handlers

import (
	"context"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"

	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type getMonthlySummaryUseCase interface {
	Execute(ctx context.Context, refMonthStr string) (dtooutput.MonthlySummary, error)
}

type GetMonthlySummaryHandler struct {
	usecase getMonthlySummaryUseCase
	o11y    observability.Observability
}

func NewGetMonthlySummaryHandler(uc getMonthlySummaryUseCase, o11y observability.Observability) *GetMonthlySummaryHandler {
	return &GetMonthlySummaryHandler{usecase: uc, o11y: o11y}
}

func (h *GetMonthlySummaryHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.get_monthly_summary")
	defer span.End()

	refMonthStr := chi.URLParam(r, "ref_month")
	if refMonthStr == "" {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "ref_month obrigatório", map[string]string{"code": "validation_error"})
		return
	}

	out, err := h.usecase.Execute(ctx, refMonthStr)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "ok"))
	responses.JSON(w, http.StatusOK, out)
}
