package handlers

import (
	"context"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/responses"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type getCardInvoiceUseCase interface {
	Execute(ctx context.Context, cardID uuid.UUID, refMonthStr string) (dtooutput.CardInvoice, error)
}

type GetCardInvoiceHandler struct {
	usecase getCardInvoiceUseCase
	o11y    observability.Observability
}

func NewGetCardInvoiceHandler(uc getCardInvoiceUseCase, o11y observability.Observability) *GetCardInvoiceHandler {
	return &GetCardInvoiceHandler{usecase: uc, o11y: o11y}
}

func (h *GetCardInvoiceHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "transactions.handler.get_card_invoice")
	defer span.End()

	cardID, err := uuid.Parse(chi.URLParam(r, "card_id"))
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "card_id inválido", map[string]string{"code": "validation_error"})
		return
	}

	refMonthStr := chi.URLParam(r, "ref_month")
	if refMonthStr == "" {
		span.SetAttributes(observability.String("outcome", "invalid_payload"))
		responses.ErrorWithDetails(w, http.StatusBadRequest, "ref_month obrigatório", map[string]string{"code": "validation_error"})
		return
	}

	out, err := h.usecase.Execute(ctx, cardID, refMonthStr)
	if err != nil {
		span.RecordError(err)
		mapError(w, span, err)
		return
	}

	span.SetAttributes(observability.String("outcome", "ok"))
	responses.JSON(w, http.StatusOK, out)
}
