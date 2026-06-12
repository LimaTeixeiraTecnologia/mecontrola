package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type deleteCardPurchaseUseCase interface {
	Execute(ctx context.Context, purchaseID uuid.UUID, version int64) error
}

type deleteCardPurchaseRequest struct {
	Version int64 `json:"version"`
}

type DeleteCardPurchaseHandler struct {
	usecase deleteCardPurchaseUseCase
	o11y    observability.Observability
}

func NewDeleteCardPurchaseHandler(uc deleteCardPurchaseUseCase, o11y observability.Observability) *DeleteCardPurchaseHandler {
	return &DeleteCardPurchaseHandler{usecase: uc, o11y: o11y}
}

func (h *DeleteCardPurchaseHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.delete_card_purchase")
	defer span.End()

	purchaseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "id inválido", map[string]string{"code": "validation_error"})
		return
	}

	var req deleteCardPurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido", map[string]string{"code": "validation_error"})
		return
	}

	if err := h.usecase.Execute(ctx, purchaseID, req.Version); err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "deleted"))
	w.WriteHeader(http.StatusNoContent)
}
