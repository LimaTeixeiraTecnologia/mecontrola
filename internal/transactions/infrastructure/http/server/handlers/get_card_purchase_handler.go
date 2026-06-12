package handlers

import (
	"context"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type getCardPurchaseUseCase interface {
	Execute(ctx context.Context, purchaseID uuid.UUID) (dtooutput.CardPurchase, error)
}

type GetCardPurchaseHandler struct {
	usecase getCardPurchaseUseCase
	o11y    observability.Observability
}

func NewGetCardPurchaseHandler(uc getCardPurchaseUseCase, o11y observability.Observability) *GetCardPurchaseHandler {
	return &GetCardPurchaseHandler{usecase: uc, o11y: o11y}
}

func (h *GetCardPurchaseHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.get_card_purchase")
	defer span.End()

	purchaseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "id inválido", map[string]string{"code": "validation_error"})
		return
	}

	out, err := h.usecase.Execute(ctx, purchaseID)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "ok"))
	responses.JSON(w, http.StatusOK, out)
}
