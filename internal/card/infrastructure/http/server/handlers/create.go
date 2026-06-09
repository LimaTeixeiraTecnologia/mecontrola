package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
)

type createCardUseCase interface {
	Execute(ctx context.Context, in input.CreateCard) (output.Card, error)
}

type createCardRequest struct {
	Name       string `json:"name"`
	Nickname   string `json:"nickname"`
	ClosingDay int    `json:"closing_day"`
	DueDay     int    `json:"due_day"`
}

type CreateCardHandler struct {
	usecase createCardUseCase
	o11y    observability.Observability
}

func NewCreateCardHandler(uc createCardUseCase, o11y observability.Observability) *CreateCardHandler {
	return &CreateCardHandler{usecase: uc, o11y: o11y}
}

func (h *CreateCardHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "card.handler.create")
	defer span.End()

	principal, _ := auth.FromContext(ctx)

	var req createCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "payload inválido",
			map[string]string{"code": "invalid_payload"})
		return
	}

	out, err := h.usecase.Execute(ctx, input.CreateCard{
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

	span.SetAttributes(
		observability.String("card_id", out.ID),
		observability.String("user_id", out.UserID),
		observability.String("outcome", "success"),
	)
	h.o11y.Logger().Info(ctx, "card.create.completed",
		observability.String("card_id", out.ID),
		observability.String("user_id", out.UserID),
	)

	w.Header().Set("Location", "/api/v1/cards/"+out.ID)
	responses.JSON(w, http.StatusCreated, out)
}

func mapCardError(w http.ResponseWriter, span observability.Span, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidCardName):
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "nome do cartão inválido",
			map[string]string{"code": "invalid_card_name"})
	case errors.Is(err, domain.ErrInvalidNickname):
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "apelido inválido",
			map[string]string{"code": "invalid_nickname"})
	case errors.Is(err, domain.ErrInvalidClosingDay):
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "dia de fechamento inválido",
			map[string]string{"code": "invalid_closing_day"})
	case errors.Is(err, domain.ErrInvalidDueDay):
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "dia de vencimento inválido",
			map[string]string{"code": "invalid_due_day"})
	case errors.Is(err, domain.ErrInvalidPurchaseDate):
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "data inválida; use YYYY-MM-DD",
			map[string]string{"code": "invalid_purchase_date"})
	case errors.Is(err, domain.ErrInvalidCursor):
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "cursor de paginação inválido",
			map[string]string{"code": "invalid_cursor"})
	case errors.Is(err, domain.ErrNicknameConflict):
		span.SetAttributes(observability.String("outcome", "conflict"))
		responses.ErrorWithDetails(w, http.StatusConflict, "apelido já em uso",
			map[string]string{"code": "nickname_in_use"})
	case errors.Is(err, domain.ErrCardNotFound):
		span.SetAttributes(observability.String("outcome", "not_found"))
		responses.ErrorWithDetails(w, http.StatusNotFound, "cartão não encontrado",
			map[string]string{"code": "card_not_found"})
	default:
		span.SetAttributes(observability.String("outcome", "internal_error"))
		responses.Error(w, http.StatusInternalServerError, "erro interno")
	}
}
