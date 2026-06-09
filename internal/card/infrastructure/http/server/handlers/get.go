package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"

	"github.com/google/uuid"
)

type getCardUseCase interface {
	Execute(ctx context.Context, in input.GetCard) (output.Card, error)
}

type GetCardHandler struct {
	usecase getCardUseCase
	o11y    observability.Observability
}

func NewGetCardHandler(uc getCardUseCase, o11y observability.Observability) *GetCardHandler {
	return &GetCardHandler{usecase: uc, o11y: o11y}
}

func (h *GetCardHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "card.handler.get")
	defer span.End()

	principal, _ := auth.FromContext(ctx)

	cardID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "id do cartão inválido",
			map[string]string{"code": "invalid_card_id"})
		return
	}

	span.SetAttributes(
		observability.String("card_id", cardID.String()),
		observability.String("user_id", principal.UserID.String()),
	)

	out, err := h.usecase.Execute(ctx, input.GetCard{
		ID:     cardID,
		UserID: principal.UserID,
	})
	if err != nil {
		span.RecordError(err)
		mapCardError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "success"))
	responses.JSON(w, http.StatusOK, out)
}
