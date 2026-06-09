package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type updateCardUseCase interface {
	Execute(ctx context.Context, in input.UpdateCard) (output.Card, error)
}

type updateCardRequest struct {
	Name       string `json:"name"`
	Nickname   string `json:"nickname"`
	ClosingDay int    `json:"closing_day"`
	DueDay     int    `json:"due_day"`
}

type UpdateCardHandler struct {
	usecase updateCardUseCase
	o11y    observability.Observability
}

func NewUpdateCardHandler(uc updateCardUseCase, o11y observability.Observability) *UpdateCardHandler {
	return &UpdateCardHandler{usecase: uc, o11y: o11y}
}

func (h *UpdateCardHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "card.handler.update")
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

	var req updateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido",
			map[string]string{"code": "invalid_payload"})
		return
	}

	out, err := h.usecase.Execute(ctx, input.UpdateCard{
		ID:         cardID,
		UserID:     principal.UserID,
		Name:       req.Name,
		Nickname:   req.Nickname,
		ClosingDay: req.ClosingDay,
		DueDay:     req.DueDay,
	})
	if err != nil {
		span.RecordError(err)
		mapCardError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "success"))
	h.o11y.Logger().Info(ctx, "card.update.completed",
		observability.String("card_id", out.ID),
		observability.String("user_id", out.UserID),
	)

	responses.JSON(w, http.StatusOK, out)
}
