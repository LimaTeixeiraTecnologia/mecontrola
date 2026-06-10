package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type getMonthlySummaryUseCase interface {
	Execute(ctx context.Context, userID string, competence string) (output.MonthlySummaryOutput, error)
}

type GetMonthlySummaryHandler struct {
	usecase getMonthlySummaryUseCase
	o11y    observability.Observability
}

func NewGetMonthlySummaryHandler(uc getMonthlySummaryUseCase, o11y observability.Observability) *GetMonthlySummaryHandler {
	return &GetMonthlySummaryHandler{usecase: uc, o11y: o11y}
}

func (h *GetMonthlySummaryHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "budgets.handler.get_monthly_summary")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		responses.Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	competence := chi.URLParam(r, "competence")
	if competence == "" {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "competence obrigatória",
			map[string]string{"code": "validation_error"})
		return
	}

	out, err := h.usecase.Execute(ctx, principal.UserID.String(), competence)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, interfaces.ErrBudgetNotFound) {
			span.SetAttributes(observability.String("outcome", "not_found"))
			responses.ErrorWithDetails(w, http.StatusNotFound, "orçamento não encontrado",
				map[string]string{"code": "budget_not_found"})
			return
		}
		span.SetAttributes(observability.String("outcome", "internal_error"))
		responses.Error(w, http.StatusInternalServerError, "erro interno")
		return
	}

	span.SetAttributes(observability.String("outcome", "ok"))
	responses.JSON(w, http.StatusOK, out)
}
