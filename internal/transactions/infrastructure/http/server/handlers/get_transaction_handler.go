package handlers

import (
	"context"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"

	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type getTransactionUseCase interface {
	Execute(ctx context.Context, txID string) (dtooutput.Transaction, error)
}

type GetTransactionHandler struct {
	usecase getTransactionUseCase
	o11y    observability.Observability
}

func NewGetTransactionHandler(uc getTransactionUseCase, o11y observability.Observability) *GetTransactionHandler {
	return &GetTransactionHandler{usecase: uc, o11y: o11y}
}

func (h *GetTransactionHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.get_transaction")
	defer span.End()

	txID := chi.URLParam(r, "id")
	if txID == "" {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "id obrigatório", map[string]string{"code": "validation_error"})
		return
	}

	out, err := h.usecase.Execute(ctx, txID)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("transaction_id", txID), observability.String("outcome", "ok"))
	responses.JSON(w, http.StatusOK, out)
}
