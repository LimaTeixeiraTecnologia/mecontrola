package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type bestPurchaseDayUseCase interface {
	Execute(ctx context.Context, in input.BestPurchaseDay) (output.BestPurchaseDay, error)
}

type BestPurchaseDayHandler struct {
	usecase bestPurchaseDayUseCase
	o11y    observability.Observability
}

func NewBestPurchaseDayHandler(uc bestPurchaseDayUseCase, o11y observability.Observability) *BestPurchaseDayHandler {
	return &BestPurchaseDayHandler{usecase: uc, o11y: o11y}
}

func (h *BestPurchaseDayHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "card.handler.best_purchase_day")
	defer span.End()

	bank := r.URL.Query().Get("bank")
	dueDayStr := r.URL.Query().Get("due_day")

	dueDay, err := strconv.Atoi(dueDayStr)
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "due_day inválido",
			map[string]string{"code": "invalid_due_day"})
		return
	}

	in := input.BestPurchaseDay{
		Bank:   bank,
		DueDay: dueDay,
	}

	if err := in.Validate(); err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		switch {
		case errors.Is(err, input.ErrCardBankRequired):
			responses.ErrorWithDetails(w, http.StatusBadRequest, "banco obrigatório",
				map[string]string{"code": "bank_required"})
		case errors.Is(err, input.ErrCardDueDayInvalid):
			responses.ErrorWithDetails(w, http.StatusBadRequest, "dia de vencimento inválido",
				map[string]string{"code": "invalid_due_day"})
		default:
			responses.ErrorWithDetails(w, http.StatusBadRequest, "parâmetros inválidos",
				map[string]string{"code": "invalid_input"})
		}
		return
	}

	out, err := h.usecase.Execute(ctx, in)
	if err != nil {
		span.RecordError(err)
		mapCardError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "success"))
	responses.JSON(w, http.StatusOK, out)
}
