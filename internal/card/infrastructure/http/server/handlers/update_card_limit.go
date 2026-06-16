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

type updateCardLimitUseCase interface {
	Execute(ctx context.Context, in input.UpdateCardLimit) (output.Card, error)
}

type updateCardLimitRequest struct {
	LimitCents      *int64 `json:"limit_cents"`
	ExpectedVersion *int64 `json:"expected_version,omitempty"`
}

type UpdateCardLimitHandler struct {
	usecase updateCardLimitUseCase
	o11y    observability.Observability
}

func NewUpdateCardLimitHandler(uc updateCardLimitUseCase, o11y observability.Observability) *UpdateCardLimitHandler {
	return &UpdateCardLimitHandler{usecase: uc, o11y: o11y}
}

func (h *UpdateCardLimitHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "card.handler.update_limit")
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

	var req updateCardLimitRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido",
			map[string]string{"code": "invalid_payload"})
		return
	}

	if req.LimitCents == nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "informe limit_cents",
			map[string]string{"code": "missing_limit_cents"})
		return
	}

	out, err := h.usecase.Execute(ctx, input.UpdateCardLimit{
		CardID:          cardID,
		UserID:          principal.UserID,
		LimitCents:      *req.LimitCents,
		ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		span.RecordError(err)
		mapCardError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "success"))
	h.o11y.Logger().Info(ctx, "card.update_limit.completed",
		observability.String("card_id", out.ID),
		observability.String("user_id", out.UserID),
	)

	responses.JSON(w, http.StatusOK, out)
}
