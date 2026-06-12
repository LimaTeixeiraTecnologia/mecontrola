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

type createTransactionUseCase interface {
	Execute(ctx context.Context, raw dtoinput.RawCreateTransaction) (dtooutput.Transaction, error)
}

type CreateTransactionHandler struct {
	usecase createTransactionUseCase
	o11y    observability.Observability
}

func NewCreateTransactionHandler(uc createTransactionUseCase, o11y observability.Observability) *CreateTransactionHandler {
	return &CreateTransactionHandler{usecase: uc, o11y: o11y}
}

func (h *CreateTransactionHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.create_transaction")
	defer span.End()

	var raw dtoinput.RawCreateTransaction
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
