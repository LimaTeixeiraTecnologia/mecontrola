package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type deleteDraftBudgetUseCase interface {
	Execute(ctx context.Context, in input.DeleteDraftInput) error
}

type DeleteBudgetHandler struct {
	usecase deleteDraftBudgetUseCase
	o11y    observability.Observability
}

func NewDeleteBudgetHandler(uc deleteDraftBudgetUseCase, o11y observability.Observability) *DeleteBudgetHandler {
	return &DeleteBudgetHandler{usecase: uc, o11y: o11y}
}

func (h *DeleteBudgetHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "budgets.handler.delete_budget")
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

	err := h.usecase.Execute(ctx, input.DeleteDraftInput{
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
			responses.ErrorWithDetails(w, http.StatusConflict, "orçamento ativo não pode ser excluído",
				map[string]string{"code": "budget_active_conflict"})
		default:
			span.SetAttributes(observability.String("outcome", "internal_error"))
			responses.Error(w, http.StatusInternalServerError, "erro interno")
		}
		return
	}

	span.SetAttributes(observability.String("outcome", "deleted"))
	w.WriteHeader(http.StatusNoContent)
}
