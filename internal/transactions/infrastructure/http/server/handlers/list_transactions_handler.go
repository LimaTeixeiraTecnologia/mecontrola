package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type listTransactionsUseCase interface {
	Execute(ctx context.Context, refMonthStr, cursor string, limit int) (usecases.TransactionPage, error)
}

type ListTransactionsHandler struct {
	usecase listTransactionsUseCase
	o11y    observability.Observability
}

func NewListTransactionsHandler(uc listTransactionsUseCase, o11y observability.Observability) *ListTransactionsHandler {
	return &ListTransactionsHandler{usecase: uc, o11y: o11y}
}

func (h *ListTransactionsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.list_transactions")
	defer span.End()

	refMonth := r.URL.Query().Get("ref_month")
	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, parseErr := strconv.Atoi(l); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}

	out, err := h.usecase.Execute(ctx, refMonth, cursor, limit)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "ok"))
	responses.JSON(w, http.StatusOK, out)
}
