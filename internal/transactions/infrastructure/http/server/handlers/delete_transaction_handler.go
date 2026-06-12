package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"
)

type deleteTransactionUseCase interface {
	Execute(ctx context.Context, txID string, version int64) error
}

type deleteTransactionRequest struct {
	Version int64 `json:"version"`
}

type DeleteTransactionHandler struct {
	usecase deleteTransactionUseCase
	o11y    observability.Observability
}

func NewDeleteTransactionHandler(uc deleteTransactionUseCase, o11y observability.Observability) *DeleteTransactionHandler {
	return &DeleteTransactionHandler{usecase: uc, o11y: o11y}
}

func (h *DeleteTransactionHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.delete_transaction")
	defer span.End()

	txID := chi.URLParam(r, "id")
	if txID == "" {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "id obrigatório", map[string]string{"code": "validation_error"})
		return
	}

	var req deleteTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido", map[string]string{"code": "validation_error"})
		return
	}

	if err := h.usecase.Execute(ctx, txID, req.Version); err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("transaction_id", txID), observability.String("outcome", "deleted"))
	w.WriteHeader(http.StatusNoContent)
}
