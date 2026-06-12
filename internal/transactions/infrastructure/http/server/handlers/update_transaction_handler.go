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

type updateTransactionUseCase interface {
	Execute(ctx context.Context, txID string, raw dtoinput.RawUpdateTransaction) (dtooutput.Transaction, error)
}

type UpdateTransactionHandler struct {
	usecase updateTransactionUseCase
	o11y    observability.Observability
}

func NewUpdateTransactionHandler(uc updateTransactionUseCase, o11y observability.Observability) *UpdateTransactionHandler {
	return &UpdateTransactionHandler{usecase: uc, o11y: o11y}
}

func (h *UpdateTransactionHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.update_transaction")
	defer span.End()

	txID := chi.URLParam(r, "id")
	if txID == "" {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "id obrigatório", map[string]string{"code": "validation_error"})
		return
	}

	var raw dtoinput.RawUpdateTransaction
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido", map[string]string{"code": "validation_error"})
		return
	}

	out, err := h.usecase.Execute(ctx, txID, raw)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("transaction_id", txID), observability.String("outcome", "updated"))
	responses.JSON(w, http.StatusOK, out)
}
