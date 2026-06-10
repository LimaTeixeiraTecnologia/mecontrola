package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type activateBudgetUseCase interface {
	Execute(ctx context.Context, in input.ActivateBudgetInput) (output.BudgetOutput, error)
}

type ActivateBudgetHandler struct {
	usecase activateBudgetUseCase
	o11y    observability.Observability
}

func NewActivateBudgetHandler(uc activateBudgetUseCase, o11y observability.Observability) *ActivateBudgetHandler {
	return &ActivateBudgetHandler{usecase: uc, o11y: o11y}
}

func (h *ActivateBudgetHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "budgets.handler.activate_budget")
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

	out, err := h.usecase.Execute(ctx, input.ActivateBudgetInput{
		UserID:     principal.UserID.String(),
		Competence: competence,
	})
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, interfaces.ErrBudgetNotFound):
			span.SetAttributes(observability.String("outcome", "not_found"))
			responses.ErrorWithDetails(w, http.StatusNotFound, "orçamento não encontrado",
				map[string]string{"code": "budget_not_found"})
		case errors.Is(err, entities.ErrBudgetAlreadyActive):
			span.SetAttributes(observability.String("outcome", "conflict"))
			responses.ErrorWithDetails(w, http.StatusConflict, "orçamento já está ativo",
				map[string]string{"code": "budget_already_active"})
		case errors.Is(err, entities.ErrBudgetTotalMustBePositive),
			errors.Is(err, entities.ErrBudgetAllocationSumMustBe10000):
			span.SetAttributes(observability.String("outcome", "unprocessable"))
			responses.ErrorWithDetails(w, http.StatusUnprocessableEntity, err.Error(),
				map[string]string{"code": "activation_invalid"})
		default:
			span.SetAttributes(observability.String("outcome", "internal_error"))
			responses.Error(w, http.StatusInternalServerError, "erro interno")
		}
		return
	}

	span.SetAttributes(observability.String("outcome", "activated"))
	responses.JSON(w, http.StatusOK, out)
}
