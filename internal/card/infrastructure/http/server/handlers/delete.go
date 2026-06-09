package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type softDeleteCardUseCase interface {
	Execute(ctx context.Context, in input.SoftDeleteCard) error
}

type DeleteCardHandler struct {
	usecase softDeleteCardUseCase
	o11y    observability.Observability
}

func NewDeleteCardHandler(uc softDeleteCardUseCase, o11y observability.Observability) *DeleteCardHandler {
	return &DeleteCardHandler{usecase: uc, o11y: o11y}
}

func (h *DeleteCardHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "card.handler.delete")
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

	if err := h.usecase.Execute(ctx, input.SoftDeleteCard{
		ID:     cardID,
		UserID: principal.UserID,
	}); err != nil {
		span.RecordError(err)
		mapCardError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "success"))
	h.o11y.Logger().Info(ctx, "card.delete.completed",
		observability.String("card_id", cardID.String()),
		observability.String("user_id", principal.UserID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}
