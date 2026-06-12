package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type listCardPurchasesUseCase interface {
	Execute(ctx context.Context, in usecases.ListCardPurchasesInput) (usecases.ListCardPurchasesOutput, error)
}

type ListCardPurchasesHandler struct {
	usecase listCardPurchasesUseCase
	o11y    observability.Observability
}

func NewListCardPurchasesHandler(uc listCardPurchasesUseCase, o11y observability.Observability) *ListCardPurchasesHandler {
	return &ListCardPurchasesHandler{usecase: uc, o11y: o11y}
}

func (h *ListCardPurchasesHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.list_card_purchases")
	defer span.End()

	in := usecases.ListCardPurchasesInput{
		Cursor: interfaces.Cursor{},
		Limit:  50,
	}

	if cardIDStr := r.URL.Query().Get("card_id"); cardIDStr != "" {
		cardID, err := uuid.Parse(cardIDStr)
		if err != nil {
			span.SetAttributes(observability.String("outcome", "invalid_payload"))
			responses.ErrorWithDetails(w, http.StatusBadRequest, "card_id inválido", map[string]string{"code": "validation_error"})
			return
		}
		in.CardID = cardID
	}

	if refMonthStr := r.URL.Query().Get("ref_month"); refMonthStr != "" {
		rm, err := valueobjects.NewRefMonth(refMonthStr)
		if err == nil {
			in.RefMonth = &rm
		}
	}

	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		in.Cursor = interfaces.Cursor{Value: cursor}
	}

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, parseErr := strconv.Atoi(l); parseErr == nil && parsed > 0 {
			in.Limit = parsed
		}
	}

	out, err := h.usecase.Execute(ctx, in)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "ok"))
	responses.JSON(w, http.StatusOK, out)
}
