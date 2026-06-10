package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type deleteExpenseUseCase interface {
	Execute(ctx context.Context, in input.DeleteExpenseInput) error
}

type deleteExpenseRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}

type DeleteExpenseHandler struct {
	usecase deleteExpenseUseCase
	o11y    observability.Observability
}

func NewDeleteExpenseHandler(uc deleteExpenseUseCase, o11y observability.Observability) *DeleteExpenseHandler {
	return &DeleteExpenseHandler{usecase: uc, o11y: o11y}
}

func (h *DeleteExpenseHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "budgets.handler.delete_expense")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		responses.Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}

	extID := chi.URLParam(r, "id")

	var req deleteExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido",
			map[string]string{"code": "invalid_payload"})
		return
	}

	err := h.usecase.Execute(ctx, input.DeleteExpenseInput{
		UserID:                principal.UserID.String(),
		Source:                "api",
		ExternalTransactionID: extID,
		ExpectedVersion:       req.ExpectedVersion,
	})
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, interfaces.ErrExpenseNotFound):
			span.SetAttributes(observability.String("outcome", "not_found"))
			responses.ErrorWithDetails(w, http.StatusNotFound, "despesa não encontrada",
				map[string]string{"code": "expense_not_found"})
		case errors.Is(err, interfaces.ErrExpenseConflict):
			span.SetAttributes(observability.String("outcome", "conflict"))
			responses.ErrorWithDetails(w, http.StatusConflict, "conflito de versão na despesa",
				map[string]string{"code": "expense_version_conflict"})
		case errors.Is(err, usecases.ErrDeleteExpenseInvalidExternalID),
			errors.Is(err, usecases.ErrDeleteExpenseInvalidSource),
			errors.Is(err, usecases.ErrDeleteExpenseInvalidUserID):
			span.SetAttributes(observability.String("outcome", "invalid"))
			responses.ErrorWithDetails(w, http.StatusBadRequest, err.Error(),
				map[string]string{"code": "validation_error"})
		default:
			span.SetAttributes(observability.String("outcome", "internal_error"))
			responses.Error(w, http.StatusInternalServerError, "erro interno")
		}
		return
	}

	span.SetAttributes(observability.String("outcome", "deleted"))
	w.WriteHeader(http.StatusNoContent)
}
