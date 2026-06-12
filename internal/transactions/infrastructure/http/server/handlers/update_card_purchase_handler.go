package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	dtoinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type updateCardPurchaseUseCase interface {
	Execute(ctx context.Context, purchaseID uuid.UUID, raw dtoinput.RawUpdateCardPurchase) (dtooutput.CardPurchase, error)
}

type UpdateCardPurchaseHandler struct {
	usecase updateCardPurchaseUseCase
	o11y    observability.Observability
}

func NewUpdateCardPurchaseHandler(uc updateCardPurchaseUseCase, o11y observability.Observability) *UpdateCardPurchaseHandler {
	return &UpdateCardPurchaseHandler{usecase: uc, o11y: o11y}
}

func (h *UpdateCardPurchaseHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.update_card_purchase")
	defer span.End()

	purchaseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "id inválido", map[string]string{"code": "validation_error"})
		return
	}

	var raw dtoinput.RawUpdateCardPurchase
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido", map[string]string{"code": "validation_error"})
		return
	}

	out, err := h.usecase.Execute(ctx, purchaseID, raw)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "updated"))
	responses.JSON(w, http.StatusOK, out)
}
