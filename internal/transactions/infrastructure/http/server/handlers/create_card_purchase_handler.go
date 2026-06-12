package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	dtoinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type createCardPurchaseUseCase interface {
	Execute(ctx context.Context, raw dtoinput.RawCreateCardPurchase) (dtooutput.CardPurchase, error)
}

type CreateCardPurchaseHandler struct {
	usecase createCardPurchaseUseCase
	o11y    observability.Observability
}

func NewCreateCardPurchaseHandler(uc createCardPurchaseUseCase, o11y observability.Observability) *CreateCardPurchaseHandler {
	return &CreateCardPurchaseHandler{usecase: uc, o11y: o11y}
}

func (h *CreateCardPurchaseHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.create_card_purchase")
	defer span.End()

	var raw dtoinput.RawCreateCardPurchase
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido", map[string]string{"code": "validation_error"})
		return
	}

	out, err := h.usecase.Execute(ctx, raw)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "created"))
	responses.JSON(w, http.StatusCreated, out)
}
